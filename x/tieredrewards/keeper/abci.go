package keeper

import (
	"context"
	"errors"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// TopUpBaseRewards implements the base rewards top-up mechanism.
func (k Keeper) TopUpBaseRewards(ctx context.Context) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get params: %v", err))
	}

	targetBaseRewardsRate := params.TargetBaseRewardsRate

	if targetBaseRewardsRate.IsZero() {
		return nil
	}

	totalBonded, err := k.stakingKeeper.TotalBondedTokens(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get total bonded tokens: %v", err))
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get bond denom: %v", err))
	}

	mintParams, err := k.mintKeeper.GetParams(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get mint params: %v", err))
	}

	blocksPerYear := mintParams.BlocksPerYear

	if blocksPerYear == 0 {
		k.Logger(ctx).Error("blocks per year is 0, skipping base rewards top up")
		return nil
	}

	communityTax, err := k.distributionKeeper.GetCommunityTax(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get community tax: %v", err))
	}

	targetStakersRewardPerBlock := math.LegacyNewDecFromInt(totalBonded).
		Mul(targetBaseRewardsRate).
		Quo(math.LegacyNewDec(int64(blocksPerYear)))

	feeCollector := k.accountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName)
	if feeCollector == nil {
		k.Logger(ctx).Error("fee collector module account not found, skipping base rewards top up")
		return nil
	}
	feeCollectorAddr := feeCollector.GetAddress()
	feeCollectorBalance := k.bankKeeper.GetBalance(ctx, feeCollectorAddr, bondDenom)
	defaultStakersRewardPerBlock := math.LegacyNewDecFromInt(feeCollectorBalance.Amount).
		MulTruncate(math.LegacyOneDec().Sub(communityTax))

	shortFallAmount := targetStakersRewardPerBlock.Sub(defaultStakersRewardPerBlock).TruncateInt()

	if !shortFallAmount.IsPositive() {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	bondedVotes := sdkCtx.VoteInfos()

	var previousTotalPower int64
	for _, voteInfo := range bondedVotes {
		previousTotalPower += voteInfo.Validator.Power
	}

	if previousTotalPower == 0 {
		k.Logger(ctx).Error("no validators are voting, skipping base rewards top up")
		return nil
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)
	topUpAmount := shortFallAmount
	if poolBalance.Amount.IsZero() {
		k.Logger(ctx).Error("base rewards pool is empty, cannot top up validator rewards",
			"shortfall", shortFallAmount.String(),
		)
		return nil
	}
	if poolBalance.Amount.LT(shortFallAmount) {
		k.Logger(ctx).Error("base rewards pool has insufficient funds, distributing remaining balance",
			"shortfall", shortFallAmount.String(),
			"pool_balance", poolBalance.Amount.String(),
		)
		topUpAmount = poolBalance.Amount
	}

	err = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.RewardsPoolName, distributiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, topUpAmount)))
	if err != nil {
		return err
	}

	topUp := sdk.NewDecCoins(sdk.NewDecCoin(bondDenom, topUpAmount))

	for _, vote := range bondedVotes {
		validator, err := k.stakingKeeper.ValidatorByConsAddr(ctx, vote.Validator.Address)
		if err != nil {
			return err
		}

		powerFraction := math.LegacyNewDec(vote.Validator.Power).QuoTruncate(math.LegacyNewDec(previousTotalPower))
		reward := topUp.MulDecTruncate(powerFraction)

		err = k.distributionKeeper.AllocateTokensToValidator(ctx, validator, reward)
		if err != nil {
			return err
		}
	}

	return sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsTopUp{
		TopUp: sdk.NewCoin(bondDenom, topUpAmount),
	})
}

// ProcessCompletedUnbondings iterates only the UnbondingPositions map (not all
// positions) and clears the unbonding flag for positions whose unbonding period
// has completed.
func (k Keeper) ProcessCompletedUnbondings(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockUnix := sdkCtx.BlockTime().Unix()

	// Collect completed IDs first to avoid mutating the map during iteration.
	var completed []uint64
	err := k.UnbondingPositions.Walk(ctx, nil, func(posId uint64, completionUnix int64) (bool, error) {
		if completionUnix == 0 || blockUnix < completionUnix {
			return false, nil
		}
		completed = append(completed, posId)
		return false, nil
	})
	if err != nil {
		return err
	}

	for _, posId := range completed {
		pos, err := k.GetPosition(ctx, posId)
		if err != nil {
			if errors.Is(err, collections.ErrNotFound) {
				// Position was deleted; clean up the stale unbonding entry.
				if removeErr := k.UnbondingPositions.Remove(ctx, posId); removeErr != nil {
					return removeErr
				}
				continue
			}
			return err
		}

		pos.IsUnbonding = false
		pos.Validator = ""
		if err := k.SetPosition(ctx, pos); err != nil {
			return err
		}

		if err := k.UnbondingPositions.Remove(ctx, posId); err != nil {
			return err
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"unbonding_complete",
			sdk.NewAttribute("position_id", fmt.Sprintf("%d", pos.PositionId)),
			sdk.NewAttribute("owner", pos.Owner),
		))
	}
	return nil
}
