package keeper_test

import (
	"time"

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

// TestCornerCase_ZeroAmountPositiveSharesState verifies that a position can end
// up with Amount == 0 while still delegated with positive shares after slash
// math; voting power should not count such zero-amount positions.
func (s *KeeperSuite) TestCornerCase_ZeroAmountPositiveSharesState() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(1000)
	lockResp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	posBefore, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().True(posBefore.DelegatedShares.IsPositive(), "test setup failed: expected positive delegated shares")

	// Slash in hook path with full fraction; in this path Amount is updated but
	// DelegatedShares are left unchanged for bonded slash handling.
	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyOneDec())
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.IsZero(), "expected position amount to be zero after slash")
	s.Require().True(posAfter.IsDelegated(), "expected position to remain delegated")
	s.Require().True(posAfter.DelegatedShares.IsPositive(), "expected delegated shares to remain positive")

	// Redelegate rejects zero amount.
	dstValAddr, _ := s.createSecondValidator()
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   lockResp.PositionId,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)

	// Zero-amount delegated positions should not count toward voting power.
	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(), "zero-amount delegated position should not contribute voting power")

	totalPower, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(totalPower.IsZero(), "zero-amount delegated positions should not contribute to total delegated voting power")
}

// TestCornerCase_ClearPositionAfterRedelegationSlashAllSharesBurnt verifies
// ClearPosition remains blocked after exit elapsed when a redelegation slash
// burns all shares and clears delegation while redelegation mapping is active.
func (s *KeeperSuite) TestCornerCase_ClearPositionAfterRedelegationSlashAllSharesBurnt() {
	delAddr, srcValAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	lockResp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       srcValAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	redelegateResp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   lockResp.PositionId,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	posBeforeSlash, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().True(posBeforeSlash.DelegatedShares.IsPositive(), "test setup failed: expected delegated shares before slash")
	s.Require().True(posBeforeSlash.IsDelegated(), "test setup failed: position should be delegated before slash")

	// Burn all shares through redelegation slash callback.
	shareBurnt := posBeforeSlash.DelegatedShares.Add(sdkmath.LegacyOneDec())
	err = s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, redelegateResp.UnbondingId, posBeforeSlash.Amount, shareBurnt)
	s.Require().NoError(err)

	posAfterSlash, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().False(posAfterSlash.IsDelegated(), "delegation should be cleared when all shares are burnt")
	s.Require().True(posAfterSlash.DelegatedShares.IsZero(), "delegated shares should be zero after full share burn")
	s.Require().True(posAfterSlash.Amount.IsZero(), "amount should be zero after full share burn")
	s.Require().True(posAfterSlash.HasTriggeredExit(), "slash should not clear exit trigger")

	// Redelegation mapping should still block ClearPosition after exit elapsed.
	redelegationIter, err := s.keeper.RedelegationMappings.Indexes.ByPosition.MatchExact(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	redelegationIDs, err := redelegationIter.PrimaryKeys()
	s.Require().NoError(err)
	s.Require().NotEmpty(redelegationIDs, "redelegation mapping should remain active for this corner case")

	s.advancePastExitDuration()

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().ErrorIs(err, types.ErrPositionRedelegation)

	posAfterClearAttempt, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().True(posAfterClearAttempt.HasTriggeredExit(), "failed clear attempt should not reset exit state")
	s.Require().False(posAfterClearAttempt.IsDelegated(), "failed clear attempt should keep cleared delegation state")
}

// TestCornerCase_ExitTriggerClearCycles_BonusAccrualCorrectness verifies that
// repeated TriggerExit/ClearPosition cycles do not double-count or under-count
// bonus accrual when cycle durations are identical.
func (s *KeeperSuite) TestCornerCase_ExitTriggerClearCycles_BonusAccrualCorrectness() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	lockResp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	cycleDuration := 24 * time.Hour

	balBeforeCycle1 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(cycleDuration))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	posAfterCycle1, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().False(posAfterCycle1.HasTriggeredExit(), "clear should reset exit state")
	s.Require().True(posAfterCycle1.IsDelegated(), "clear cycle should keep delegated position active")
	s.Require().Equal(s.ctx.BlockTime(), posAfterCycle1.LastBonusAccrual, "clear should checkpoint bonus accrual at current time")

	balAfterCycle1 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	cycle1Payout := balAfterCycle1.Amount.Sub(balBeforeCycle1.Amount)
	s.Require().True(cycle1Payout.IsPositive(), "first cycle should pay positive bonus")

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(cycleDuration))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: lockResp.PositionId,
	})
	s.Require().NoError(err)

	posAfterCycle2, err := s.keeper.GetPosition(s.ctx, lockResp.PositionId)
	s.Require().NoError(err)
	s.Require().False(posAfterCycle2.HasTriggeredExit(), "clear should reset exit state in repeated cycle")
	s.Require().True(posAfterCycle2.IsDelegated(), "repeated clear should keep delegated position active")
	s.Require().Equal(s.ctx.BlockTime(), posAfterCycle2.LastBonusAccrual, "repeated clear should checkpoint to current time")

	balAfterCycle2 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	cycle2Payout := balAfterCycle2.Amount.Sub(balAfterCycle1.Amount)
	s.Require().True(cycle2Payout.IsPositive(), "second cycle should pay positive bonus")
	s.Require().True(cycle2Payout.Equal(cycle1Payout),
		"equal-duration cycles should pay equal bonus, got cycle1=%s cycle2=%s", cycle1Payout, cycle2Payout)
}
