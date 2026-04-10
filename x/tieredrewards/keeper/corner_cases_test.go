package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// TestCornerCase_StaleValidatorRewardRatioReplayed verifies stale validator
// ratio is cleared when module delegation on that validator reaches zero, so a
// later delegation lifecycle cannot replay historical base rewards.
func (s *KeeperSuite) TestCornerCase_StaleValidatorRewardRatioReplayed() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	// First lifecycle: create position and leave a non-zero validator ratio.
	lockResp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	_, err = msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	ratioAfterFirstClaim, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(ratioAfterFirstClaim.IsZero(), "test setup failed: expected a non-zero ratio after first claim")

	// Fully close the first position so module delegation on the validator is removed.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr)

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().Error(err, "expected no module delegation after position withdrawal")

	staleRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(staleRatio.IsZero(), "test setup failed: expected historical ratio before re-entry")

	// Second lifecycle: create a fresh position with no new rewards allocated.
	freshAddr := sdk.AccAddress([]byte("stale_ratio_new_user__"))
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, freshAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	lockResp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	ratioAfterReentry, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ratioAfterReentry.IsZero(), "stale validator ratio should be reset on re-entry when no module delegation exists")

	// Ensure the module account has spendable balance so replayed stale ratio can
	// be observed as an actual overpayment, not masked by insufficient-funds.
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000))))
	s.Require().NoError(err)

	// No new rewards were allocated for this second lifecycle.
	claimResp2, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      freshAddr.String(),
		PositionId: lockResp2.PositionId,
	})
	s.Require().NoError(err)

	s.Require().True(
		claimResp2.BaseRewards.AmountOf(bondDenom).IsZero(),
		"second lifecycle claim should not replay historical base rewards when no new rewards were allocated",
	)
}

