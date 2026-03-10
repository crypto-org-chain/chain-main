package tieredrewards

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	return topUpBaseRewards(ctx, k)
}

func topUpBaseRewards(ctx context.Context, k keeper.Keeper) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get params: %v", err))
	}

	targetBaseRewardsRate := params.TargetBaseRewardsRate

	if targetBaseRewardsRate.IsZero() {
		return nil
	}

	totalBonded, err := k.TotalBondedTokens(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get total bonded tokens: %v", err))
	}

	bondDenom, err := k.BondDenom(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get bond denom: %v", err))
	}

	blocksPerYear, err := k.GetBlocksPerYear(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get blocks per year: %v", err))
	}

	communityTax, err := k.GetCommunityTax(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get community tax: %v", err))
	}

	targetStakersReward := math.LegacyNewDecFromInt(totalBonded).
		Mul(targetBaseRewardsRate).
		Quo(math.LegacyNewDec(int64(blocksPerYear)))

	feeCollectorAddr := k.GetModuleAccount(ctx, authtypes.FeeCollectorName).GetAddress()
	feeCollectorBalance := k.GetBalance(ctx, feeCollectorAddr, bondDenom)
	defaultStakersReward := math.LegacyNewDecFromInt(feeCollectorBalance.Amount).
		MulTruncate(math.LegacyOneDec().Sub(communityTax))

	shortFallAmount := targetStakersReward.Sub(defaultStakersReward).TruncateInt()

	if !shortFallAmount.IsPositive() {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	bondedVotes := sdkCtx.VoteInfos()

	var previousTotalPower int64
	for _, voteInfo := range bondedVotes {
		previousTotalPower += voteInfo.Validator.Power
	}

	// if no validators are voting, skip
	if previousTotalPower == 0 {
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
