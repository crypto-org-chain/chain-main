package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMsgTierRedelegate_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create a second validator to redelegate to
	dstValAddr, _ := s.createSecondValidator()
	// Create delegated position

	resp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(dstValAddr.String(), pos.Delegation.ValidatorAddress)
	s.Require().True(pos.Delegation.Shares.IsPositive())
	s.Require().Equal(uint64(0), pos.LastEventSeq, "LastEventSeq should be 0 for fresh destination validator")

	// Verify the redelegating-position reverse mapping was populated.
	isRedelegating, err := s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(isRedelegating, "redelegating position mapping should be populated after TierRedelegate")
}

func (s *KeeperSuite) TestMsgTierRedelegate_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Undelegate first so position is not delegated
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierRedelegate_SameValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create delegated position

	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrRedelegationToSameValidator)
}

// TestMsgTierRedelegate_ExitInProgress verifies that TierRedelegate succeeds
// when exit is in progress.
func (s *KeeperSuite) TestMsgTierRedelegate_ExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Exit is triggered but NOT elapsed.
	pos, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(dstValAddr.String(), pos.Delegation.ValidatorAddress)
	s.Require().True(pos.HasTriggeredExit())
}

// TestMsgTierRedelegate_ExitElapsed verifies that TierRedelegate is rejected
// when exit has fully elapsed — user must ClearPosition first.
func (s *KeeperSuite) TestMsgTierRedelegate_ExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.advancePastExitDuration()
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Exit has elapsed, position still delegated.
	pos, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationElapsed)
}

func (s *KeeperSuite) TestMsgTierRedelegate_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        wrongAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgTierRedelegate_UpdatesValidatorIndex() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Position count should be attributed to source validator
	srcCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), srcCount)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	// Source validator count should be cleared (entry removed).
	srcCount, err = s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), srcCount)

	// Destination validator should own the position now.
	dstCount, err := s.keeper.GetPositionCountForValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), dstCount)
}

// TestMsgTierRedelegate_ClaimsRewardsBeforeRedelegating verifies that TierRedelegate
// claims pending rewards before performing the redelegation. A subsequent ClaimTierRewards
// call (with no new rewards allocated) should yield zero base rewards.
func (s *KeeperSuite) TestMsgTierRedelegate_ClaimsRewardsBeforeRedelegating() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	dstValAddr, _ := s.createSecondValidator()

	// Advance time and allocate rewards so there are pending base rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierRedelegate internally claims rewards.
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	balAfterRedelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterRedelegate.Amount.GT(balBefore.Amount), "rewards should be paid during redelegate")

	// No new rewards allocated — subsequent ClaimTierRewards on dst validator should yield zero base.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should already be claimed during redelegate")
}

// TestMsgTierRedelegate_TierCloseOnly verifies that TierRedelegate is rejected
// when the tier is set to CloseOnly.
func (s *KeeperSuite) TestMsgTierRedelegate_TierCloseOnly() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Set tier to close only.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

// TestMsgTierRedelegate_TransitiveRedelegation verifies that the tier-level
// guard blocks a second redelegation while the first is still pending
func (s *KeeperSuite) TestMsgTierRedelegate_TransitiveRedelegation() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	srcValAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)

	// First redelegate: A → B.
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(dstValAddr.String(), pos.Delegation.ValidatorAddress, "position should be on validator B")

	// Second redelegate: B → A while A→B is still pending.
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: srcValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrActiveRedelegation)
}

// TestPositionCountByValidator_Redelegate verifies that when a position is
// redelegated from valA to valB, valA's count decreases and valB's count increases.
func (s *KeeperSuite) TestPositionCountByValidator_Redelegate() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	srcValAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	// Source validator should have count=1.
	srcCount, err := s.keeper.PositionCountByValidator.Get(s.ctx, srcValAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), srcCount, "source validator should have 1 position before redelegate")

	dstValAddr, _ := s.createSecondValidator()

	// Destination validator should have count=0 (not found).
	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, dstValAddr)
	s.Require().Error(err, "destination validator should have no positions before redelegate")

	// Redelegate.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	// Source validator count should decrease to 0 (removed from store).
	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, srcValAddr)
	s.Require().Error(err, "source validator should have no positions after redelegate")

	// Destination validator count should be 1.
	dstCount, err := s.keeper.PositionCountByValidator.Get(s.ctx, dstValAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), dstCount, "destination validator should have 1 position after redelegate")
}

// TestMsgTierRedelegate_DstValidatorNotBonded verifies that redelegation
// to a non-bonded (unbonding/unbonded) validator is blocked.
func (s *KeeperSuite) TestMsgTierRedelegate_DstValidatorNotBonded() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	dstValAddr, _ := s.createSecondValidator()

	// Jail the destination validator to make it unbonding.
	s.jailAndUnbondValidator(dstValAddr)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)

	// Position should remain on the original validator.
	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(valAddr.String(), posAfter.Delegation.ValidatorAddress, "position should stay on original validator")
}

// TestMsgTierRedelegate_FromUnbondingSrc verifies that TierRedelegate succeeds
// when the source validator is Unbonding (jailed but unbonding period not yet elapsed).
// Unlike the fully-Unbonded case, a redelegation entry IS created (unbondingId > 0)
// and a RedelegationMappings entry is stored.
func (s *KeeperSuite) TestMsgTierRedelegate_FromUnbondingSrc() {
	srcValAddr, _ := s.createSecondValidator()

	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()*2))
	s.Require().NoError(s.keeper.LockFunds(s.ctx, freshAddr, types.GetDelegatorAddress(1), sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())))

	s.setupTier(1)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()),
		ValidatorAddress: srcValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPositionState(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// Jail src validator → transitions to Unbonding (not yet Unbonded).
	s.jailAndUnbondValidator(srcValAddr)

	srcVal, err := s.app.StakingKeeper.GetValidator(s.ctx, srcValAddr)
	s.Require().NoError(err)
	s.Require().True(srcVal.IsUnbonding(), "src validator should be Unbonding, not yet Unbonded")

	// Redelegate: Unbonding src → bonded genesis validator.
	vals, _ := s.getStakingData()
	dstValAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	redResp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        freshAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().False(redResp.CompletionTime.IsZero(), "completion time should be set for unbonding src")

	// Position should now be on the destination validator.
	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(dstValAddr.String(), posAfter.Delegation.ValidatorAddress)
	s.Require().True(posAfter.Delegation.Shares.IsPositive())

	// Reverse mapping should contain the entry (unlike the fully-Unbonded case).
	isRedelegating, err := s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(isRedelegating, "redelegation mapping should exist for unbonding src redelegate")

	// Attempt transitive redelegation: dst → src while src→dst is still pending.
	// Unjail src so it's bonded again and eligible as a destination.
	consAddr, _ := srcVal.GetConsAddr()
	s.Require().NoError(s.app.StakingKeeper.Unjail(s.ctx, consAddr))
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        freshAddr.String(),
		PositionId:   pos.Id,
		DstValidator: srcValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrActiveRedelegation,
		"second redelegate should be blocked by the tier-level isRedelegating guard")
}

// TestMsgTierRedelegate_FromUnbondedSrc_NoMapping verifies that when the source
// validator is fully Unbonded.
// A second redelegate succeeds immediately (no transitive redelegation block).
func (s *KeeperSuite) TestMsgTierRedelegate_FromUnbondedSrc_NoMapping() {
	// Create second validator as source — we'll unbond it fully.
	srcValAddr, _ := s.createSecondValidator()

	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()*2))
	s.Require().NoError(s.keeper.LockFunds(s.ctx, freshAddr, types.GetDelegatorAddress(1), sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())))

	s.setupTier(1)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()),
		ValidatorAddress: srcValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPositionState(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// Jail src validator → Unbonding status.
	s.jailAndUnbondValidator(srcValAddr)

	// Advance time past unbonding period to transition src to Unbonded.
	unbondingTime, err := s.app.StakingKeeper.UnbondingTime(s.ctx)
	s.Require().NoError(err)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(unbondingTime + time.Second))
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.Require().NoError(s.app.StakingKeeper.UnbondAllMatureValidators(s.ctx))

	// Verify src is now Unbonded.
	srcVal, err := s.app.StakingKeeper.GetValidator(s.ctx, srcValAddr)
	s.Require().NoError(err)
	s.Require().True(srcVal.IsUnbonded(), "src validator should be fully Unbonded")

	// First redelegate: Unbonded src → bonded genesis validator.
	vals, _ := s.getStakingData()
	dstValAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        freshAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	// No mapping for the unbonded-src validator
	isRedelegating, err := s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(isRedelegating, "no reverse mapping should be written when src is fully unbonded")

	// Unjail src so it's bonded again for the second redelegate destination.
	consAddr, _ := srcVal.GetConsAddr()
	s.Require().NoError(s.app.StakingKeeper.Unjail(s.ctx, consAddr))
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	// Second redelegate: genesis → src should succeed immediately
	// (no transitive block since no redelegation entry exists from first redelegate).
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        freshAddr.String(),
		PositionId:   pos.Id,
		DstValidator: srcValAddr.String(),
	})
	s.Require().NoError(err, "second redelegate should succeed — no transitive block")

	// Position should now be on src validator.
	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(srcValAddr.String(), posAfter.Delegation.ValidatorAddress)
}

// TestMsgTierRedelegate_MultiplePositionsNoTransitiveBlock exercises the core
// benefit of the per-position-delegator rewrite: one position's pending
// redelegation (A → B) must not block a different position from redelegating
// out of B.
func (s *KeeperSuite) TestMsgTierRedelegate_MultiplePositionsNoTransitiveBlock() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valA := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	valB, _ := s.createSecondValidator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)

	lockAmount := sdkmath.NewInt(1_000)

	// pos1 locks on validator A.
	owner1 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, owner1,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            owner1.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valA.String(),
	})
	s.Require().NoError(err)
	pos1Id := resp1.PositionId

	// pos2 locks on validator B.
	owner2 := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, owner2,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            owner2.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valB.String(),
	})
	s.Require().NoError(err)
	pos2Id := resp2.PositionId

	// pos1 redelegates A → B.
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        owner1.String(),
		PositionId:   pos1Id,
		DstValidator: valB.String(),
	})
	s.Require().NoError(err)

	// pos2 redelegates B → A.
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        owner2.String(),
		PositionId:   pos2Id,
		DstValidator: valA.String(),
	})
	s.Require().NoError(err, "pos2 B→A must not be blocked by pos1's pending A→B redelegation")

	pos1, err := s.keeper.GetPositionState(s.ctx, pos1Id)
	s.Require().NoError(err)
	s.Require().Equal(valB.String(), pos1.Delegation.ValidatorAddress)
	pos2, err := s.keeper.GetPositionState(s.ctx, pos2Id)
	s.Require().NoError(err)
	s.Require().Equal(valA.String(), pos2.Delegation.ValidatorAddress)
}
