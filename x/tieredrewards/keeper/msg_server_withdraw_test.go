package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMsgWithdrawFromTier_Basic_Undelegated() {
	lockAmount := sdkmath.NewInt(1000)
	// Lock tokens with delegation and immediate exit trigger
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Advance time past exit unlock (tier exit duration is 365 days)
	s.advancePastExitDuration()

	// Fund bonus pool before undelegation
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Undelegate first
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Advance time past staking unbonding period and complete unbonding so
	// the staking module returns tokens to the tier module account.
	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount),
		"response should include withdrawn amount")

	// Owner should have received the locked tokens back
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(lockAmount)),
		"owner should have received locked tokens back")

	// Position should be deleted
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_PositionDeletedFromIndexes() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Verify position exists in owner index
	posIds, err := s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)

	// Verify position count for tier
	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Advance time and undelegate
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Advance time past staking unbonding period and complete unbonding so
	// the staking module returns tokens to the tier module account.
	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Owner index should be empty
	posIds, err = s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Empty(posIds, "owner index should be empty after withdrawal")

	// Position count for tier should be 0
	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count, "tier position count should be 0 after withdrawal")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_ExitNotTriggered() {
	// Lock tokens without triggering exit
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_ExitCommitmentNotElapsed() {
	// Lock tokens with immediate exit trigger
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Don't advance time — exit commitment hasn't elapsed
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_StillDelegated() {
	// Lock with delegation and immediate exit trigger
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Advance time past exit unlock
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	// Try to withdraw while still delegated (haven't undelegated)
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionDelegated)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_PositionNotFound() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	delAddr := sdk.AccAddress([]byte("some_address________"))
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 999,
	})
	s.Require().Error(err)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_AfterUndelegate() {
	// Lock with delegation and immediate exit trigger
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Fund the rewards pool so bonus claim in undelegate doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Advance past exit duration so undelegation is allowed.
	s.advancePastExitDuration()
	undelegateResp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(undelegateResp.CompletionTime.IsZero())

	var mappingExistsBeforeWithdraw bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, positionId uint64) (bool, error) {
		if positionId == 0 {
			mappingExistsBeforeWithdraw = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(mappingExistsBeforeWithdraw, "unbonding mapping should exist before withdrawal")

	// Position should not be delegated but still exists
	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())

	// Advance time past exit unlock (365 days + 1 day) for withdrawal
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Now withdraw — requires exit commitment elapsed and unbonding completed
	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount))

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(lockAmount)),
		"owner should have received locked tokens back after undelegate + withdraw")

	// Position should be deleted
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")

	var mappingExistsAfterWithdraw bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, positionId uint64) (bool, error) {
		if positionId == 0 {
			mappingExistsAfterWithdraw = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().False(mappingExistsAfterWithdraw, "unbonding mapping should be cleaned after position deletion")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_MultiplePositions_WithdrawOne() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	// Create two positions with immediate exit
	lockAmt2 := sdkmath.NewInt(2000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmt2,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Advance time past exit
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Undelegate both positions
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 1,
	})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	// Withdraw only the first position
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// First position should be deleted
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "first position should be deleted")

	// Second position should still exist
	pos2, err := s.keeper.GetPosition(s.ctx, uint64(1))
	s.Require().NoError(err)
	s.Require().True(sdkmath.NewInt(2000).Equal(pos2.Amount))

	// Owner should still have 1 position in index
	posIds, err := s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)
	s.Require().Equal(uint64(1), posIds[0])

	// Tier should have 1 position remaining
	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)
}

// TestMsgWithdrawFromTier_AfterUndelegate_NoInsolvency verifies the full
// lifecycle: lock → delegate → exit → undelegate → withdraw. The module
// account should have exactly enough tokens for withdrawal after the
// reconciliation fix, without needing extra manual funding.
func (s *KeeperSuite) TestMsgWithdrawFromTier_AfterUndelegate_NoInsolvency() {
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash to get non-1:1 rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))

	lockAmount := sdkmath.NewInt(10001)
	pos := s.setupNewTierPosition(lockAmount, true)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)

	s.advancePastExitDuration()
	// Undelegate — this updates pos.Amount with actual return value.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Advance time past exit unlock.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(366 * 24 * time.Hour))

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	// Withdrawal should succeed — the module has exactly enough tokens.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(pos.Amount.String(), resp.Amount.AmountOf(bondDenom).String(),
		"withdrawn amount should equal amount")
}

// TestWithdrawFromTier_FailsWithPendingUnbonding verifies that withdrawal is
// blocked when unbonding entries are still pending (mapping exists).
func (s *KeeperSuite) TestWithdrawFromTier_FailsWithPendingUnbonding() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(100000), bondDenom)

	s.advancePastExitDuration()
	// Undelegate — this creates an unbonding mapping.
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Verify unbonding mapping exists.
	hasUnbonding, err := s.keeper.StillUnbonding(s.ctx, 0)
	s.Require().NoError(err)
	s.Require().True(hasUnbonding, "unbonding mapping should exist after TierUndelegate")

	// Advance time past exit lock duration.
	s.advancePastExitDuration()

	// Withdrawal should fail because unbonding entries are pending.
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionUnbonding)

	hooks := s.keeper.Hooks()
	// Get the unbonding IDs for position 0.
	iter, err := s.keeper.UnbondingDelegationMappings.Indexes.ByPosition.MatchExact(s.ctx, uint64(0))
	s.Require().NoError(err)
	unbondingIds, err := iter.PrimaryKeys()
	s.Require().NoError(err)
	s.Require().NotEmpty(unbondingIds)

	// Simulate unbonding completion via hook.
	posDelAddr := types.GetDelegatorAddress(pos.Id)
	err = hooks.AfterUnbondingCompleted(s.ctx, posDelAddr, valAddr, unbondingIds)
	s.Require().NoError(err)

	// Verify mapping is cleaned up.
	hasUnbonding, err = s.keeper.StillUnbonding(s.ctx, 0)
	s.Require().NoError(err)
	s.Require().False(hasUnbonding, "unbonding mapping should be cleaned up after hook")

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	// Withdrawal should now pass since unbonding entries are cleaned up.
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
}

// TestMsgWithdrawFromTier_RedelegSlashedToZero verifies the full lifecycle of
// a position that was zeroed by a redelegation slash: trigger exit, advance
// past exit duration, then withdraw. The zero-amount withdrawal should succeed
// and delete the position.
func (s *KeeperSuite) TestMsgWithdrawFromTier_RedelegSlashedToZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash: clear delegation and zero amount.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Trigger exit and advance past exit duration.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.advancePastExitDuration()

	// Withdraw — zero-amount position should be cleanly deleted.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.IsZero(),
		"withdrawn amount should be zero")

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

// TestMsgWithdrawFromTier_TierCloseOnly_Succeeds verifies that WithdrawFromTier
// is NOT blocked by CloseOnly — exit-path messages must always succeed.
func (s *KeeperSuite) TestMsgWithdrawFromTier_TierCloseOnly_Succeeds() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Undelegate first.
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Complete unbonding.
	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	// Set tier to CloseOnly.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	// WithdrawFromTier — should succeed despite CloseOnly.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).IsPositive(),
		"withdrawal should return locked tokens")

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

// TestMsgWithdrawFromTier_UnbondingSlashedToZero verifies withdrawal of a
// position whose unbonding delegation was slashed to zero. The position should
// be cleanly deleted with zero returned.
func (s *KeeperSuite) TestMsgWithdrawFromTier_UnbondingSlashedToZero() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Undelegate — creates unbonding delegation.
	undelegateResp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Simulate unbonding slash to zero via hook.
	s.slashUnbondingEntry(types.GetDelegatorAddress(pos.Id), valAddr, undelegateResp.UnbondingId, pos.Amount)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.Amount.IsZero(), "position amount should be zero after unbonding slash")

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	// Withdraw — zero-amount position should be cleanly deleted.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).IsZero(),
		"withdrawn amount should be zero")

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

// TestMsgWithdrawFromTier_UnbondingSlashedPartial verifies withdrawal of a
// position whose unbonding delegation was partially slashed (50%). The
// position should be deleted and the reduced amount returned to the owner.
func (s *KeeperSuite) TestMsgWithdrawFromTier_UnbondingSlashedPartial() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Undelegate — creates unbonding delegation.
	undelegateResp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Simulate 50% unbonding slash via hook.
	slashAmount := pos.Amount.QuoRaw(2)
	s.slashUnbondingEntry(types.GetDelegatorAddress(pos.Id), valAddr, undelegateResp.UnbondingId, slashAmount)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	expectedRemaining := lockAmount.Sub(slashAmount)
	s.Require().Equal(expectedRemaining.String(), pos.Amount.String(),
		"position amount should reflect partial slash")

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Withdraw — should return the reduced amount.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(expectedRemaining.String(), resp.Amount.AmountOf(bondDenom).String(),
		"withdrawn amount should equal post-slash remainder")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(expectedRemaining.String(), balAfter.Amount.Sub(balBefore.Amount).String(),
		"owner balance increase should match withdrawn amount")

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

// TestMsgWithdrawFromTier_UndelegatedWithFunds verifies the lifecycle of a
// position that was zeroed by a redelegation slash, then replenished via
// AddToTier (without re-delegation), then exited. The full added amount
// should be returned on withdrawal.
func (s *KeeperSuite) TestMsgWithdrawFromTier_UndelegatedWithFunds() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash: clear delegation and zero amount.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Add 2000 to position (undelegated — funds go to module account, not staked).
	addAmount := sdkmath.NewInt(2000)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().NoError(err)

	// Trigger exit and advance past exit duration.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.advancePastExitDuration()

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Withdraw — should return the 2000 added funds.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(addAmount),
		"withdrawn amount should equal the 2000 added")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(addAmount)),
		"owner should receive 2000 back")

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

// TestMsgWithdrawFromTier_SweepsNonBondDenomDust verifies that anything stray
// left on the position's delegator account — not just the bond denom principal —
// is swept to the owner on withdrawal. Dust of this kind shouldn't occur in
// practice (rewards route to the owner; staking only moves bondDenom), but
// WithdrawFromTier is defensively tolerant so no denom gets orphaned.
func (s *KeeperSuite) TestMsgWithdrawFromTier_SweepsNonBondDenomDust() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      ownerAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posDelAddr := types.GetDelegatorAddress(pos.Id)
	s.completeStakingUnbonding(valAddr, posDelAddr)

	// Inject dust of an arbitrary denom directly onto the position's delegator
	// account, simulating stray coins that ended up there.
	dustDenom := "dust"
	dustAmount := sdkmath.NewInt(777)
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, posDelAddr,
		sdk.NewCoins(sdk.NewCoin(dustDenom, dustAmount))))

	dustBefore := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, dustDenom)

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      ownerAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount),
		"response should report the bond-denom principal")
	s.Require().True(resp.Amount.AmountOf(dustDenom).Equal(dustAmount),
		"response should report swept dust of any denom")

	dustAfter := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, dustDenom)
	s.Require().True(dustAfter.Amount.Equal(dustBefore.Amount.Add(dustAmount)),
		"owner should have received the dust too")

	s.Require().True(s.app.BankKeeper.GetAllBalances(s.ctx, posDelAddr).IsZero(),
		"position's delegator account should be empty after sweep")
}

// TestMsgWithdrawFromTier_ClearsWithdrawAddrRouting verifies deletePosition's
// removeBaseRewardsRouting step wipes the distribution DelegatorsWithdrawAddress
// entry set at position creation.
func (s *KeeperSuite) TestMsgWithdrawFromTier_ClearsWithdrawAddrRouting() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Sanity: at creation time the withdraw-addr points at the owner.
	withdrawAddr, err := s.app.DistrKeeper.GetDelegatorWithdrawAddr(s.ctx, posDelAddr)
	s.Require().NoError(err)
	s.Require().Equal(ownerAddr.String(), withdrawAddr.String(),
		"withdraw addr should route to owner before position is deleted")

	// Drive the position through to withdrawal.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      ownerAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.completeStakingUnbonding(valAddr, posDelAddr)

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      ownerAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// After deletion the mapping must be gone. Distribution returns the
	// delegator address itself as the default when no mapping is stored.
	withdrawAddrAfter, err := s.app.DistrKeeper.GetDelegatorWithdrawAddr(s.ctx, posDelAddr)
	s.Require().NoError(err)
	s.Require().Equal(posDelAddr.String(), withdrawAddrAfter.String(),
		"withdraw-addr mapping should be cleared on position deletion; "+
			"GetDelegatorWithdrawAddr should fall back to the delegator itself")
}
