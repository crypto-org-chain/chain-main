package keeper

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

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

// collectValidatorDelegationRewards pulls accumulated staking distribution rewards from
// x/distribution into the tier module account for the delegation to valAddr.
func (k Keeper) collectValidatorDelegationRewards(ctx context.Context, valAddr sdk.ValAddress) (rewards sdk.Coins, collected bool, err error) {
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

	rewards, collected, err := k.collectValidatorDelegationRewards(ctx, valAddr)
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

// calculateBonus returns accrued bonus, yielding zero when the validator is not bonded.
func (k Keeper) calculateBonus(position types.Position, validator stakingtypes.Validator, tier types.Tier, blockTime time.Time) math.Int {
	if !validator.IsBonded() {
		return math.ZeroInt()
	}
	return k.calculateBonusRaw(position, validator, tier, blockTime)
}

// calculateBonusRaw computes accrued bonus without checking validator status.
// Formula: tokens * BonusApy * durationSeconds / SecondsPerYear.
// accrualEnd is capped at ExitUnlockAt when the position is exiting.
func (k Keeper) calculateBonusRaw(position types.Position, validator stakingtypes.Validator, tier types.Tier, blockTime time.Time) math.Int {
	if !position.IsDelegated() {
		return math.ZeroInt()
	}

	if position.LastBonusAccrual.IsZero() {
		return math.ZeroInt()
	}

	accrualEnd := blockTime
	if position.CompletedExitLockDuration(blockTime) {
		accrualEnd = position.ExitUnlockAt
	}

	if !accrualEnd.After(position.LastBonusAccrual) {
		return math.ZeroInt()
	}

	durationSeconds := int64(accrualEnd.Sub(position.LastBonusAccrual) / time.Second)
	tokens := validator.TokensFromShares(position.DelegatedShares)

	return tokens.
		Mul(tier.BonusApy).
		MulInt64(durationSeconds).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()
}

// settleRewardsForPositions settles base and bonus rewards for a batch of
// positions on the same validator. It is designed for hook paths (slash,
// unbonding, bonding) where the caller must proceed even if the bonus pool
// is insufficient. When the bonus pool cannot cover a position's accrued
// bonus, the checkpoint is still advanced and the position persisted so the
// accrual window is consumed — the unpaid bonus is forfeited.
func (k Keeper) settleRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position, forceAccrue bool) error {
	currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return err
	}

	validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return err
	}

	tierCache := make(map[uint32]types.Tier)

	for i := range positions {
		if _, err := k.claimBaseRewards(ctx, &positions[i], currentRatio); err != nil {
			return err
		}

		tier, ok := tierCache[positions[i].TierId]
		if !ok {
			tier, err = k.getTier(ctx, positions[i].TierId)
			if err != nil {
				return err
			}
			tierCache[positions[i].TierId] = tier
		}

		_, err := k.claimBonusRewardsWithValidator(ctx, &positions[i], validator, tier, forceAccrue)
		if err != nil {
			if errors.Is(err, types.ErrInsufficientBonusPool) {
				k.logger(ctx).Error(err.Error())
			} else {
				return err
			}
		}
		// Checkpoint is already advanced in-memory by claimBonusRewardsWithValidator.
		// Persist regardless of whether bonus was paid.
		if err := k.setPosition(ctx, positions[i]); err != nil {
			return err
		}
	}

	return nil
}

// claimRewardsForTier claims base and bonus rewards for all delegated
// positions in the given tier.
func (k Keeper) claimRewardsForTier(ctx context.Context, tierId uint32) error {
	positions, err := k.getPositionsByTier(ctx, tierId)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	// Group delegated positions by validator because updateBaseRewardsPerShare
	// must be called once per validator batch.
	byValidator := make(map[string][]types.Position)
	for _, pos := range positions {
		if !pos.IsDelegated() {
			continue
		}
		byValidator[pos.Validator] = append(byValidator[pos.Validator], pos)
	}

	for valAddrStr, valPositions := range byValidator {
		valAddr, err := sdk.ValAddressFromBech32(valAddrStr)
		if err != nil {
			return err
		}

		currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
		if err != nil {
			return err
		}

		validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return err
		}

		tier, err := k.getTier(ctx, tierId)
		if err != nil {
			return err
		}

		for i := range valPositions {
			if _, err := k.claimBaseRewards(ctx, &valPositions[i], currentRatio); err != nil {
				return err
			}
			if _, err := k.claimBonusRewardsWithValidator(ctx, &valPositions[i], validator, tier, false); err != nil {
				return err
			}
			if err := k.setPosition(ctx, valPositions[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

// claimAndRefreshPosition claims rewards for a single position and returns the
// updated in-memory value so callers can finish their mutation and persist once.
//
// Returns:
//   - refreshed: the updated position with reward checkpoints advanced;
//   - base: base rewards paid to the owner for this position in this call;
//   - bonus: bonus rewards paid to the owner for this position in this call;
func (k Keeper) claimAndRefreshPosition(ctx context.Context, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	if !pos.IsDelegated() {
		return pos, sdk.NewCoins(), sdk.NewCoins(), nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	base, err := k.claimBaseRewards(ctx, &pos, currentRatio)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	bonus, err := k.claimBonusRewards(ctx, &pos, tier, false)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	return pos, base, bonus, nil
}

// claimBaseRewards calculates and sends a position's accrued base rewards.
// reward = DelegatedShares * (currentRatio - BaseRewardsPerShare)
func (k Keeper) claimBaseRewards(ctx context.Context, pos *types.Position, currentRatio sdk.DecCoins) (sdk.Coins, error) {
	if !pos.IsDelegated() {
		return sdk.Coins{}, nil
	}

	delta, hasNegative := currentRatio.SafeSub(pos.BaseRewardsPerShare)
	if hasNegative {
		k.logger(ctx).Error(
			"difference in base rewards per share is negative, keeping previous checkpoint",
			"position", pos.String(),
			"current_ratio", currentRatio.String(),
			"delta", delta.String(),
		)
		panic("negative base rewards per share delta")
	}
	pos.UpdateBaseRewardsPerShare(currentRatio)

	if delta.IsZero() {
		return sdk.Coins{}, nil
	}

	posRewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()
	if posRewards.IsZero() {
		return sdk.Coins{}, nil
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.Coins{}, err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, posRewards); err != nil {
		return sdk.Coins{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     posRewards,
	}); err != nil {
		return sdk.Coins{}, err
	}

	return posRewards, nil
}

func applyBonusAccrualCheckpoint(pos *types.Position, blockTime time.Time) {
	accrualEnd := blockTime
	if pos.CompletedExitLockDuration(blockTime) {
		accrualEnd = pos.ExitUnlockAt
	}
	pos.UpdateLastBonusAccrual(accrualEnd)
}

// bonusAccrualAmount returns bonus owed for pos at blockTime. When forceAccrue is true,
// bonded status is ignored (calculateBonusRaw).
func (k Keeper) bonusAccrualAmount(pos types.Position, val stakingtypes.Validator, tier types.Tier, blockTime time.Time, forceAccrue bool) math.Int {
	if forceAccrue {
		return k.calculateBonusRaw(pos, val, tier, blockTime)
	}
	return k.calculateBonus(pos, val, tier, blockTime)
}

// sendBonusFromRewardsPool checks the rewards pool, transfers bonus to the owner, and emits EventBonusRewardsClaimed.
func (k Keeper) sendBonusFromRewardsPool(ctx context.Context, pos types.Position, bonus math.Int) (sdk.Coins, error) {
	if bonus.IsZero() {
		return sdk.Coins{}, nil
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return sdk.Coins{}, err
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)

	if poolBalance.Amount.LT(bonus) {
		return sdk.Coins{}, errorsmod.Wrapf(types.ErrInsufficientBonusPool, "bonus pool has insufficient funds, position id: %d, bonus: %s, pool balance: %s", pos.Id, bonus.String(), poolBalance.Amount.String())
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.Coins{}, err
	}

	bonusCoin := sdk.NewCoin(bondDenom, bonus)
	bonusCoins := sdk.NewCoins(bonusCoin)
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, bonusCoins); err != nil {
		return sdk.Coins{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBonusRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     bonusCoin,
	}); err != nil {
		return sdk.Coins{}, err
	}

	return bonusCoins, nil
}

// claimBonusRewards calculates and pays the bonus for a position from the rewards pool.
// When forceAccrue is true, bonus is settled regardless of validator bonded status.
func (k Keeper) claimBonusRewards(ctx context.Context, pos *types.Position, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return sdk.Coins{}, err
	}

	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	return k.claimBonusRewardsWithValidator(ctx, pos, val, tier, forceAccrue)
}

func (k Keeper) claimBonusRewardsWithValidator(ctx context.Context, pos *types.Position, val stakingtypes.Validator, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
	blockTime := sdk.UnwrapSDKContext(ctx).BlockTime()

	bonus := k.bonusAccrualAmount(*pos, val, tier, blockTime, forceAccrue)
	applyBonusAccrualCheckpoint(pos, blockTime)

	return k.sendBonusFromRewardsPool(ctx, *pos, bonus)
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
