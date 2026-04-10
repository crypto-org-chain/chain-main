package keeper

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (k Keeper) withdrawDelegationRewards(ctx context.Context, valAddr sdk.ValAddress) (sdk.Coins, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	return k.distributionKeeper.WithdrawDelegationRewards(ctx, moduleAddr, valAddr)
}

func (k Keeper) getValidatorRewardRatio(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	ratio, err := k.ValidatorRewardRatio.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		return sdk.DecCoins{}, nil
	}
	if err != nil {
		return sdk.DecCoins{}, err
	}
	return ratio.CumulativeRewardsPerShare, nil
}

// collectDelegationRewards pulls accumulated staking distribution rewards from
// x/distribution into the tier module account for the delegation to valAddr.
func (k Keeper) collectDelegationRewards(ctx context.Context, valAddr sdk.ValAddress) (rewards sdk.Coins, collected bool, err error) {
	currentBlockHeight := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
	if lastBlock := k.getLastRewardsWithdrawalBlock(ctx, valAddr); lastBlock == currentBlockHeight {
		return sdk.Coins{}, false, nil
	}

	rewards, err = k.withdrawDelegationRewards(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, false, err
	}

	k.setLastRewardsWithdrawalBlock(ctx, valAddr, currentBlockHeight)

	return rewards, true, nil
}

// updateBaseRewardsPerShare withdraws base rewards from x/distribution and
// updates the cumulative rewards-per-share ratio for the given validator.
// Must be called before any operation that changes the module's delegation shares.
func (k Keeper) updateBaseRewardsPerShare(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	currentRatio, err := k.getValidatorRewardRatio(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	delegation, err := k.stakingKeeper.GetDelegation(ctx, poolAddr, valAddr)
	if errors.Is(err, stakingtypes.ErrNoDelegation) {
		return sdk.DecCoins{}, nil
	}
	if err != nil {
		return sdk.DecCoins{}, err
	}

	totalShares := delegation.Shares
	if totalShares.IsZero() {
		return sdk.DecCoins{}, nil
	}

	rewards, collected, err := k.collectDelegationRewards(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}
	if !collected || rewards.IsZero() {
		return currentRatio, nil
	}

	decRewards := sdk.NewDecCoinsFromCoins(rewards...)
	delta := decRewards.QuoDecTruncate(totalShares)

	newRatio := currentRatio.Add(delta...)

	err = k.ValidatorRewardRatio.Set(ctx, valAddr, types.ValidatorRewardRatio{
		CumulativeRewardsPerShare: newRatio,
	})
	if err != nil {
		return sdk.DecCoins{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsPerShareUpdated{
		Validator:                 valAddr.String(),
		RewardsWithdrawn:          rewards,
		CumulativeRewardsPerShare: newRatio,
	}); err != nil {
		return sdk.DecCoins{}, err
	}

	return newRatio, nil
}

// getLastRewardsWithdrawalBlock reads the last withdrawal block height for a validator
// from the transient store. Returns 0 if not set (never withdrawn this block).
func (k Keeper) getLastRewardsWithdrawalBlock(ctx context.Context, valAddr sdk.ValAddress) uint64 {
	store := k.transientStoreService.OpenTransientStore(ctx)
	bz, err := store.Get(valAddr)
	if err != nil || bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

// setLastRewardsWithdrawalBlock writes the last withdrawal block height for a validator
// to the transient store. The value is automatically cleared at the end of the block.
func (k Keeper) setLastRewardsWithdrawalBlock(ctx context.Context, valAddr sdk.ValAddress, blockHeight uint64) {
	store := k.transientStoreService.OpenTransientStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, blockHeight)
	if err := store.Set(valAddr, bz); err != nil {
		panic(fmt.Errorf("failed to set last rewards withdrawal block: %w", err))
	}
}
