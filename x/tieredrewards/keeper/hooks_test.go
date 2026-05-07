package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// --- BeforeValidatorSlashed hook tests ---

// TestBeforeValidatorSlashed_RecordsSlashEvent verifies that the hook records
// a SLASH event with the correct fraction instead of eagerly slashing positions.
func (s *KeeperSuite) TestBeforeValidatorSlashed_RecordsSlashEvent() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	slashFraction := sdkmath.LegacyNewDecWithPrec(1, 2) // 1% slash
	hooks := s.keeper.Hooks()
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	// Verify event was recorded.
	currentSeq, err := s.keeper.ValidatorEventSeq.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), currentSeq, "current-seq should be 1 after one event appended (first is 1)")

	evt, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, evt.EventType)
}

// TestBeforeValidatorSlashed_DoesNotModifyPosition verifies that the hook
// does NOT modify position state (lazy approach).
func (s *KeeperSuite) TestBeforeValidatorSlashed_DoesNotModifyPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	posBefore, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Position delegation shares should not be modified by the hook.
	s.Require().Equal(posBefore.Delegation.Shares, posAfter.Delegation.Shares,
		"position DelegatedShares should not change during slash hook (lazy)")
	s.Require().Equal(posBefore.LastBonusAccrual, posAfter.LastBonusAccrual,
		"position LastBonusAccrual should not change during slash hook (lazy)")
}

// TestBeforeValidatorSlashed_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestBeforeValidatorSlashed_NoPositions() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	hooks := s.keeper.Hooks()

	// Should not error when there are no positions.
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)
}

// TestBeforeValidatorSlashed_FullSlash_DoesNotHaltChain verifies that a 100% slash
// records an event without error.
func (s *KeeperSuite) TestBeforeValidatorSlashed_FullSlash_DoesNotHaltChain() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	s.setupNewTierPosition(lockAmount, false)
	pos, err := s.keeper.GetPositionState(s.ctx, uint64(0))
	s.Require().NoError(err)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	hooks := s.keeper.Hooks()
	// 100% slash must not error.
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyOneDec())
	s.Require().NoError(err, "100% slash should not cause an error")

	// Event should be recorded.
	evt, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, evt.EventType)
}

// TestBeforeValidatorSlashed_MultiplePositions verifies that the hook records
// a single event for multiple positions.
func (s *KeeperSuite) TestBeforeValidatorSlashed_MultiplePositions() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, bondDenom := s.getStakingData()
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount.MulRaw(2))))
	s.Require().NoError(err)

	for range 2 {
		_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
			Owner:            delAddr.String(),
			Id:               1,
			Amount:           lockAmount,
			ValidatorAddress: valAddr.String(),
		})
		s.Require().NoError(err)
	}

	positions, err := s.keeper.GetPositionStatesByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 3)

	slashFraction := sdkmath.LegacyNewDecWithPrec(10, 2) // 10%
	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	// Event recorded with reference count matching position count.
	evt, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(uint64(3), evt.ReferenceCount,
		"reference count should equal number of positions")

	// Positions should NOT have their delegation shares modified.
	for _, p := range positions {
		posAfter, err := s.keeper.GetPositionState(s.ctx, p.Id)
		s.Require().NoError(err)
		s.Require().Equal(p.Delegation.Shares, posAfter.Delegation.Shares,
			"position %d DelegatedShares should not change during slash hook", p.Id)
	}
}

// --- AfterValidatorBonded hook tests ---

// TestAfterValidatorBonded_RecordsBondEvent verifies that when a validator
// transitions to bonded, a BOND event is recorded instead of iterating positions.
func (s *KeeperSuite) TestAfterValidatorBonded_RecordsBondEvent() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	newTime := s.ctx.BlockTime().Add(time.Hour * 48)
	s.ctx = s.ctx.WithBlockTime(newTime)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	// Verify BOND event was recorded.
	evt, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_BOND, evt.EventType)
	s.Require().Equal(newTime, evt.Timestamp)
}

// TestAfterValidatorBonded_DoesNotModifyPosition verifies that the hook does NOT
// modify position LastBonusAccrual (lazy approach).
func (s *KeeperSuite) TestAfterValidatorBonded_DoesNotModifyPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	posBefore, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	newTime := s.ctx.BlockTime().Add(time.Hour * 48)
	s.ctx = s.ctx.WithBlockTime(newTime)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err = hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(posBefore.LastBonusAccrual, posAfter.LastBonusAccrual,
		"LastBonusAccrual should NOT be modified by AfterValidatorBonded hook (lazy)")
}

// TestAfterValidatorBonded_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestAfterValidatorBonded_NoPositions() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)
}

// --- AfterValidatorBeginUnbonding hook tests ---

// TestAfterValidatorBeginUnbonding_RecordsUnbondEvent verifies that when a validator
// begins unbonding, an UNBOND event is recorded instead of claiming rewards.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_RecordsUnbondEvent() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBeginUnbonding(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	// Verify UNBOND event was recorded.
	evt, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_UNBOND, evt.EventType)
}

// TestAfterValidatorBeginUnbonding_DoesNotModifyPosition verifies that the hook
// does NOT modify position state (lazy approach).
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_DoesNotModifyPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	posBefore, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err = hooks.AfterValidatorBeginUnbonding(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(posBefore.LastBonusAccrual, posAfter.LastBonusAccrual,
		"LastBonusAccrual should NOT be modified by unbonding hook (lazy)")
}

// --- AfterValidatorRemoved hook tests ---

// TestAfterValidatorRemoved_NoEvents_CleansSeq verifies that when no events
// remain, the hook clears the event seq counter.
func (s *KeeperSuite) TestAfterValidatorRemoved_NoEvents_CleansSeq() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()

	// Set seq counter but NO events (simulating all events garbage-collected).
	err := s.keeper.ValidatorEventSeq.Set(s.ctx, valAddr, uint64(5))
	s.Require().NoError(err)

	err = hooks.AfterValidatorRemoved(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	// Seq should be cleaned (no leftover events).
	hasSeq, err := s.keeper.ValidatorEventSeq.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasSeq, "event seq should be cleaned when no events remain")
}

// TestAfterValidatorRemoved_LeftoverEvents_PreservesSeqAndEvents verifies that
// when leftover events exist, the hook logs an error but preserves events and seq.
func (s *KeeperSuite) TestAfterValidatorRemoved_LeftoverEvents_PreservesSeqAndEvents() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()

	// Seed a leftover event and seq.
	sdkCtx := sdk.UnwrapSDKContext(s.ctx)
	err := s.keeper.ValidatorEvents.Set(s.ctx, collections.Join(valAddr, uint64(1)), types.ValidatorEvent{
		Height:         sdkCtx.BlockHeight(),
		Timestamp:      sdkCtx.BlockTime(),
		EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
		TokensPerShare: sdkmath.LegacyOneDec(),
		ReferenceCount: 1,
	})
	s.Require().NoError(err)
	err = s.keeper.ValidatorEventSeq.Set(s.ctx, valAddr, uint64(1))
	s.Require().NoError(err)

	err = hooks.AfterValidatorRemoved(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	// Events should be PRESERVED (not deleted).
	hasEvent, err := s.keeper.ValidatorEvents.Has(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().True(hasEvent, "leftover events should be preserved")

	// Seq should be PRESERVED.
	hasSeq, err := s.keeper.ValidatorEventSeq.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(hasSeq, "event seq should be preserved when leftover events exist")
}

// --- AfterRedelegationCompleted hook tests ---

func (s *KeeperSuite) TestAfterRedelegationCompleted_DeletesMapping() {
	hooks := s.keeper.Hooks()

	positionId := uint64(2)
	unbondingID := uint64(42)
	s.Require().NoError(s.keeper.RedelegationMappings.Set(s.ctx, unbondingID, positionId))

	has, err := s.keeper.RedelegationMappings.Has(s.ctx, unbondingID)
	s.Require().NoError(err)
	s.Require().True(has)

	delAddr := types.GetDelegatorAddress(positionId)
	valSrc := sdk.ValAddress([]byte("validator_src_______"))
	valDst := sdk.ValAddress([]byte("validator_dst_______"))
	err = hooks.AfterRedelegationCompleted(s.ctx, delAddr, valSrc, valDst, []uint64{unbondingID})
	s.Require().NoError(err)

	has, err = s.keeper.RedelegationMappings.Has(s.ctx, unbondingID)
	s.Require().NoError(err)
	s.Require().False(has, "redelegation mapping should be removed after completion")
}

func (s *KeeperSuite) TestAfterRedelegationCompleted_NoMapping_NoOp() {
	hooks := s.keeper.Hooks()

	delAddr := types.GetDelegatorAddress(1)
	valSrc := sdk.ValAddress([]byte("validator_src_______"))
	valDst := sdk.ValAddress([]byte("validator_dst_______"))
	err := hooks.AfterRedelegationCompleted(s.ctx, delAddr, valSrc, valDst, []uint64{99})
	s.Require().NoError(err, "should not error when unbonding id has no mapping")
}

// --- NoOp callbacks ---

func (s *KeeperSuite) TestHooks_NoOpCallbacks_ReturnNil() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	delAddr := sdk.AccAddress([]byte("noop_delegator_addr"))

	hooks := s.keeper.Hooks()
	s.Require().NoError(hooks.AfterUnbondingInitiated(s.ctx, 1))
	s.Require().NoError(hooks.BeforeValidatorModified(s.ctx, valAddr))
	s.Require().NoError(hooks.AfterValidatorCreated(s.ctx, valAddr))
	s.Require().NoError(hooks.BeforeDelegationCreated(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.BeforeDelegationSharesModified(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.BeforeDelegationRemoved(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.AfterDelegationModified(s.ctx, delAddr, valAddr))
}
