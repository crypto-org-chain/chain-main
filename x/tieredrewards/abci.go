package tieredrewards

import (
	"context"
	"errors"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	return topUpBaseRewards(ctx, k)
}

// EndBlocker processes completed unbondings for tier positions (CRIT-2).
func EndBlocker(ctx context.Context, k keeper.Keeper) error {
	return processCompletedUnbondings(ctx, k)
}

// processCompletedUnbondings iterates only the UnbondingPositions map (not all
// positions) and clears the unbonding flag for positions whose unbonding period
// has completed.
func processCompletedUnbondings(ctx context.Context, k keeper.Keeper) error {
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

// topUpBaseRewards implements the base rewards top-up mechanism.
// 6a: All panics replaced with error returns.
func topUpBaseRewards(ctx context.Context, k keeper.Keeper) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get params: %w", err)
	}

	targetBaseRewardsRate := params.TargetBaseRewardsRate

	if targetBaseRewardsRate.IsZero() {
		return nil
	}

	totalBonded, err := k.TotalBondedTokens(ctx)
	if err != nil {
		return fmt.Errorf("failed to get total bonded tokens: %w", err)
	}

	bondDenom, err := k.BondDenom(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bond denom: %w", err)
	}

	mintParams, err := k.GetMintParams(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get mint params: %v", err))
	}

	blocksPerYear := mintParams.BlocksPerYear

	if blocksPerYear == 0 {
		k.Logger(ctx).Error("blocks per year is 0, skipping base rewards top up")
		return nil
	}

	communityTax, err := k.GetCommunityTax(ctx)
	if err != nil {
		return fmt.Errorf("failed to get community tax: %w", err)
	}

	targetStakersRewardPerBlock := math.LegacyNewDecFromInt(totalBonded).
		Mul(targetBaseRewardsRate).
		Quo(math.LegacyNewDec(int64(blocksPerYear)))

	feeCollector := k.GetModuleAccount(ctx, authtypes.FeeCollectorName)
	if feeCollector == nil {
		k.Logger(ctx).Error("fee collector module account not found, skipping base rewards top up")
		return nil
	}
	feeCollectorAddr := feeCollector.GetAddress()
	feeCollectorBalance := k.GetBalance(ctx, feeCollectorAddr, bondDenom)
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

	poolAddr := k.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.GetBalance(ctx, poolAddr, bondDenom)
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

	err = k.SendCoinsFromModuleToModule(ctx, types.RewardsPoolName, distributiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, topUpAmount)))
	if err != nil {
		return err
	}

	topUp := sdk.NewDecCoins(sdk.NewDecCoin(bondDenom, topUpAmount))

	for _, vote := range bondedVotes {
		validator, err := k.ValidatorByConsAddr(ctx, vote.Validator.Address)
		if err != nil {
			return err
		}

		powerFraction := math.LegacyNewDec(vote.Validator.Power).QuoTruncate(math.LegacyNewDec(previousTotalPower))
		reward := topUp.MulDecTruncate(powerFraction)

		err = k.AllocateTokensToValidator(ctx, validator, reward)
		if err != nil {
			return err
		}
	}

	return sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsTopUp{
		TopUp: sdk.NewCoin(bondDenom, topUpAmount),
	})
}
