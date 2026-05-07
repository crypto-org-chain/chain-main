package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMsgClearPosition_ClearsExitAndAllowsAddToTier() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	addPosAmt := sdkmath.NewInt(500)
	_, bondDenom := s.getStakingData()
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrPositionTriggeredExit)

	clearResp, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), clearResp.PositionId)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit())

	valCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)
}

// TestMsgClearPosition_UpdatesLastBonusAccrualAfterExitElapsed verifies that
// ClearPosition past exit duration updates LastBonusAccrual to the current
// block time, confirming reward settlement occurred.
func (s *KeeperSuite) TestMsgClearPosition_UpdatesLastBonusAccrualAfterExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Advance past exit duration
	s.advancePastExitDuration()

	_, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit())
	s.Require().Equal(s.ctx.BlockTime(), pos.LastBonusAccrual,
		"last_bonus_accrual should equal block time after ClearPosition")
}

func (s *KeeperSuite) TestMsgClearPosition_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgClearPosition_NoOpWhenNotExiting() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Create claimable rewards and advance bonus accrual time to catch unintended side-effects.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000), bondDenom)

	posBefore, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Isolate events from this specific call.
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.ClearPosition(freshCtx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	s.Require().False(pos.HasTriggeredExit(), "position should still not be exiting")
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount), "clearing a non-exiting position must not claim rewards")
	s.Require().Equal(posBefore.LastBonusAccrual, pos.LastBonusAccrual, "clearing a non-exiting position must not mutate accrual state")

	foundExitCleared := false
	for _, e := range freshCtx.EventManager().Events() {
		if e.Type == "chainmain.tieredrewards.v1.EventExitCleared" {
			foundExitCleared = true
			break
		}
	}
	s.Require().False(foundExitCleared, "clearing a non-exiting position must not emit EventExitCleared")
}

// TestMsgClearPosition_RejectsWhileUnbonding verifies that ClearPosition is
// rejected when the position is unbonding.
func (s *KeeperSuite) TestMsgClearPosition_RejectsWhileUnbonding() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	// Position is still unbonding
	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionUnbonding)
}

// TestMsgClearPosition_RejectsAfterUnbondingCompleted verifies that ClearPosition
// is rejected on an undelegated position after unbonding completes.
func (s *KeeperSuite) TestMsgClearPosition_RejectsAfterUnbondingCompleted() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

// TestMsgClearPosition_AllowsPendingRedelegationWhenStillDelegated verifies that
// ClearPosition can clear exit after exit elapsed even if the staking-layer
// redelegation is still pending, so long as the position remains delegated.
func (s *KeeperSuite) TestMsgClearPosition_AllowsPendingRedelegationWhenStillDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	posDelAddr := types.GetDelegatorAddress(pos.Id)
	_ = posDelAddr
	isRedelegating, err := s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(isRedelegating, "redelegation mapping should exist after TierRedelegate")

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit(), "clear should reset exit state")
	s.Require().True(pos.IsDelegated(), "position should remain delegated on the destination validator")
	s.Require().Equal(dstValAddr.String(), pos.Delegation.ValidatorAddress)

	isRedelegating, err = s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(isRedelegating, "clearing exit should not delete pending redelegation tracking")
}

// TestClearPositionAfterRedelegationSlashAllSharesBurnt verifies
// ClearPosition remains blocked after exit elapsed when a redelegation slash
// burns all shares and clears delegation while redelegation mapping is active.
func (s *KeeperSuite) TestClearPositionAfterRedelegationSlashAllSharesBurnt() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	posBeforeSlash, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posBeforeSlash.Delegation.Shares.IsPositive(), "test setup failed: expected delegated shares before slash")
	s.Require().True(posBeforeSlash.IsDelegated(), "test setup failed: position should be delegated before slash")

	// Simulate the full-share burn that a redelegation slash would perform.
	// slashRedelegationCompletely removes the staking delegation and resets
	// bonus checkpoints on the tier position.
	s.slashRedelegationCompletely(posBeforeSlash)

	posAfterSlash, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfterSlash.IsDelegated(), "delegation should be cleared when all shares are burnt")
	s.Require().Nil(posAfterSlash.Delegation, "delegation pointer should be nil after full share burn")
	s.Require().True(s.getPositionAmount(posAfterSlash).IsZero(), "amount should be zero after full share burn")
	s.Require().True(posAfterSlash.HasTriggeredExit(), "slash should not clear exit trigger")

	// Redelegation mapping stays active here, but the clear failure reason is that
	// the slash isRedelegating already cleared delegation from the position.
	isRedelegating, err := s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(isRedelegating, "redelegation mapping should remain active for this corner case")

	s.advancePastExitDuration()

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)

	posAfterClearAttempt, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfterClearAttempt.HasTriggeredExit(), "failed clear attempt should not reset exit state")
	s.Require().False(posAfterClearAttempt.IsDelegated(), "failed clear attempt should keep cleared delegation state")
}

// TestMsgClearPosition_TierCloseOnly verifies that ClearPosition is rejected
// when the tier is set to CloseOnly.
func (s *KeeperSuite) TestMsgClearPosition_TierCloseOnly() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set tier to close only.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

// TestMsgClearPosition_BondedZeroExitInProgress verifies that ClearPosition
// succeeds on a delegated position with zero amount whose exit is in progress
// (not yet elapsed). The exit flag should be cleared.
func (s *KeeperSuite) TestMsgClearPosition_BondedZeroExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash validator 100% to zero out position amount via hook.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	pos, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should be zero after 100%% slash")
	s.Require().True(pos.IsDelegated(), "position should still be delegated")

	// Trigger exit.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Clear exit — should succeed even with zero amount.
	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit(), "exit should be cleared")
	s.Require().True(pos.IsDelegated(), "position should still be delegated")
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should still be zero")
}

// TestMsgClearPosition_UndelegatedZeroExitInProgress verifies that ClearPosition
// succeeds on an undelegated position with zero amount whose exit is in progress
// (not yet elapsed).
func (s *KeeperSuite) TestMsgClearPosition_UndelegatedZeroExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash: clear delegation and zero amount.
	pos = s.slashRedelegationCompletely(pos)

	// Trigger exit on the undelegated-zero position.
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit(), "position should be exiting")
	s.Require().False(pos.IsDelegated(), "position should be undelegated")
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should be zero")

	// ClearPosition — should succeed even on undelegated-zero exiting position.
	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit(), "exit should be cleared")
	s.Require().False(pos.IsDelegated(), "position should still be undelegated")
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should still be zero")
}

// TestMsgClearPosition_UndelegatedWithFundsExitInProgress verifies that
// ClearPosition succeeds on an undelegated position with funds whose exit is
// in progress (not yet elapsed).
func (s *KeeperSuite) TestMsgClearPosition_UndelegatedWithFundsExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash: clear delegation and zero amount.
	pos = s.slashRedelegationCompletely(pos)

	// Add funds to the undelegated position.
	addAmount := sdkmath.NewInt(2000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().NoError(err)

	// Trigger exit.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit(), "position should be exiting")
	s.Require().False(pos.IsDelegated(), "position should be undelegated")
	s.Require().True(s.getPositionAmount(pos).Equal(addAmount), "position amount should be 2000")

	// ClearPosition — should succeed.
	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit(), "exit should be cleared")
	s.Require().False(pos.IsDelegated(), "position should still be undelegated")
	s.Require().True(s.getPositionAmount(pos).Equal(addAmount), "position amount should still be 2000")
}

// TestMsgClearPosition_BondedZeroExitElapsed verifies that ClearPosition
// succeeds on a delegated position with zero amount after exit has elapsed.
// The exit flag should be cleared and the position remains delegated.
func (s *KeeperSuite) TestMsgClearPosition_BondedZeroExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash validator 100% to zero out position amount via hook.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	pos, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should be zero after 100%% slash")

	// Advance past exit duration.
	s.advancePastExitDuration()

	// Clear exit — should succeed on delegated position past exit duration.
	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit(), "exit should be cleared")
	s.Require().True(pos.IsDelegated(), "position should still be delegated")
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should still be zero")
}
