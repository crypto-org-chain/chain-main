package keeper

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func (k Keeper) BeginBlocker(ctx context.Context) error {
	return k.topUpBaseRewards(ctx)
}

// topUpBaseRewards ensures per-block staker rewards meet TargetBaseRewardsRate.
// If fee-collector balance (after community tax) implies a shortfall, it transfers
// the difference from the rewards pool to distribution and allocates pro-rata
// by last-block consensus voting power.
func (k Keeper) topUpBaseRewards(ctx context.Context) error {
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
		k.logger(ctx).Error("blocks per year is 0, skipping base rewards top up")
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
		k.logger(ctx).Error("fee collector module account not found, skipping base rewards top up")
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
		k.logger(ctx).Error("no validators are voting, skipping base rewards top up")
		return nil
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)
	topUpAmount := shortFallAmount
	if poolBalance.Amount.IsZero() {
		k.logger(ctx).Error("base rewards pool is empty, cannot top up validator rewards",
			"shortfall", shortFallAmount.String(),
		)
		return nil
	}
	if poolBalance.Amount.LT(shortFallAmount) {
		k.logger(ctx).Error("base rewards pool has insufficient funds, distributing remaining balance",
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
	remaining := topUp

	for i, vote := range bondedVotes {
		validator, err := k.stakingKeeper.ValidatorByConsAddr(ctx, vote.Validator.Address)
		if err != nil {
			return err
		}

		var reward sdk.DecCoins
		if i == len(bondedVotes)-1 {
			// Give remainder to last validator to avoid untracked dust
			// in the distribution module from power-fraction truncation.
			reward = remaining
		} else {
			powerFraction := math.LegacyNewDec(vote.Validator.Power).QuoTruncate(math.LegacyNewDec(previousTotalPower))
			reward = topUp.MulDecTruncate(powerFraction)
			remaining = remaining.Sub(reward)
		}

		err = k.distributionKeeper.AllocateTokensToValidator(ctx, validator, reward)
		if err != nil {
			return err
		}
	}

	return sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsTopUp{
		TopUp: sdk.NewCoin(bondDenom, topUpAmount),
	})
}
