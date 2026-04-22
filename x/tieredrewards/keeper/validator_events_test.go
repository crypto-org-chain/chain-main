package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) makeEvent(eventType types.ValidatorEventType, refCount uint64) types.ValidatorEvent {
	return types.ValidatorEvent{
		Height:         s.ctx.BlockHeight(),
		Timestamp:      s.ctx.BlockTime(),
		EventType:      eventType,
		TokensPerShare: sdkmath.LegacyOneDec(),
		ReferenceCount: refCount,
	}
}

// --- appendValidatorEvent ---

func (s *KeeperSuite) TestAppendValidatorEvent_FirstEvent() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
	seq, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), seq, "first event should get seq 1")

	// NextSeq should be 2.
	nextSeq, err := s.keeper.ValidatorEventNextSeq.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), nextSeq)

	// Event should be stored.
	stored, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(evt.EventType, stored.EventType)
	s.Require().Equal(evt.ReferenceCount, stored.ReferenceCount)
}

func (s *KeeperSuite) TestAppendValidatorEvent_SequentialSeqs() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	for i := uint64(1); i <= 5; i++ {
		evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
		seq, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
		s.Require().NoError(err)
		s.Require().Equal(i, seq, "seq should increment")
	}

	nextSeq, err := s.keeper.ValidatorEventNextSeq.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(6), nextSeq)
}

// --- getValidatorEventLatestSeq ---

func (s *KeeperSuite) TestGetValidatorEventLatestSeq_NoEvents() {
	valAddr := sdk.ValAddress([]byte("val_no_events_______"))

	latestSeq, err := s.keeper.GetValidatorEventLatestSeq(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), latestSeq, "no events → latest seq should be 0")
}

func (s *KeeperSuite) TestGetValidatorEventLatestSeq_AfterAppend() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_BOND, 1)
	_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
	s.Require().NoError(err)
	_, err = s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
	s.Require().NoError(err)

	latestSeq, err := s.keeper.GetValidatorEventLatestSeq(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), latestSeq, "after 2 appends, latest seq should be 2")
}

// --- getValidatorEventsSince ---

func (s *KeeperSuite) TestGetValidatorEventsSince_NoEvents() {
	valAddr := sdk.ValAddress([]byte("val_no_events_______"))

	entries, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr, 0)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *KeeperSuite) TestGetValidatorEventsSince_ReturnsOnlyAfterStartSeq() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	// Append 3 events.
	for i := 0; i < 3; i++ {
		s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
		evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
		_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
		s.Require().NoError(err)
	}

	// Since seq 0 → all 3.
	entries, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr, 0)
	s.Require().NoError(err)
	s.Require().Len(entries, 3)
	s.Require().Equal(uint64(1), entries[0].Seq)
	s.Require().Equal(uint64(2), entries[1].Seq)
	s.Require().Equal(uint64(3), entries[2].Seq)

	// Since seq 1 → events 2 and 3.
	entries, err = s.keeper.GetValidatorEventsSince(s.ctx, valAddr, 1)
	s.Require().NoError(err)
	s.Require().Len(entries, 2)
	s.Require().Equal(uint64(2), entries[0].Seq)
	s.Require().Equal(uint64(3), entries[1].Seq)

	// Since seq 3 → none.
	entries, err = s.keeper.GetValidatorEventsSince(s.ctx, valAddr, 3)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *KeeperSuite) TestGetValidatorEventsSince_IsolatedByValidator() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr1 := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	valAddr2, _ := s.createSecondValidator()

	// 2 events on val1, 1 event on val2.
	evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
	_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr1, evt)
	s.Require().NoError(err)
	_, err = s.keeper.AppendValidatorEvent(s.ctx, valAddr1, evt)
	s.Require().NoError(err)
	_, err = s.keeper.AppendValidatorEvent(s.ctx, valAddr2, evt)
	s.Require().NoError(err)

	entries1, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr1, 0)
	s.Require().NoError(err)
	s.Require().Len(entries1, 2, "val1 should have 2 events")

	entries2, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr2, 0)
	s.Require().NoError(err)
	s.Require().Len(entries2, 1, "val2 should have 1 event")
}

// --- decrementEventRefCount ---

func (s *KeeperSuite) TestDecrementEventRefCount_DecrementsByOne() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 3)
	_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
	s.Require().NoError(err)

	err = s.keeper.DecrementEventRefCount(s.ctx, valAddr, 1)
	s.Require().NoError(err)

	stored, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), stored.ReferenceCount)
}

func (s *KeeperSuite) TestDecrementEventRefCount_DeletesAtZero() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
	_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
	s.Require().NoError(err)

	err = s.keeper.DecrementEventRefCount(s.ctx, valAddr, 1)
	s.Require().NoError(err)

	// Should be deleted.
	has, err := s.keeper.ValidatorEvents.Has(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	s.Require().False(has, "event should be garbage-collected when refCount reaches 0")
}

func (s *KeeperSuite) TestDecrementEventRefCount_NoOpForMissingEvent() {
	valAddr := sdk.ValAddress([]byte("val_no_events_______"))

	// Should not error on non-existent event.
	err := s.keeper.DecrementEventRefCount(s.ctx, valAddr, 999)
	s.Require().NoError(err)
}

// --- deleteAllValidatorEvents ---

func (s *KeeperSuite) TestDeleteAllValidatorEvents_CleansEverything() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	// Append 3 events.
	for i := 0; i < 3; i++ {
		evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
		_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr, evt)
		s.Require().NoError(err)
	}

	// Verify they exist.
	entries, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr, 0)
	s.Require().NoError(err)
	s.Require().Len(entries, 3)

	hasNextSeq, err := s.keeper.ValidatorEventNextSeq.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(hasNextSeq)

	// Delete all.
	err = s.keeper.DeleteAllValidatorEvents(s.ctx, valAddr)
	s.Require().NoError(err)

	// All events gone.
	entries, err = s.keeper.GetValidatorEventsSince(s.ctx, valAddr, 0)
	s.Require().NoError(err)
	s.Require().Empty(entries, "all events should be deleted")

	// NextSeq gone.
	hasNextSeq, err = s.keeper.ValidatorEventNextSeq.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasNextSeq, "next seq should be deleted")
}

func (s *KeeperSuite) TestDeleteAllValidatorEvents_DoesNotAffectOtherValidators() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr1 := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	valAddr2, _ := s.createSecondValidator()

	evt := s.makeEvent(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, 1)
	_, err := s.keeper.AppendValidatorEvent(s.ctx, valAddr1, evt)
	s.Require().NoError(err)
	_, err = s.keeper.AppendValidatorEvent(s.ctx, valAddr2, evt)
	s.Require().NoError(err)

	// Delete val1 events only.
	err = s.keeper.DeleteAllValidatorEvents(s.ctx, valAddr1)
	s.Require().NoError(err)

	// Val1 empty.
	entries1, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr1, 0)
	s.Require().NoError(err)
	s.Require().Empty(entries1)

	// Val2 untouched.
	entries2, err := s.keeper.GetValidatorEventsSince(s.ctx, valAddr2, 0)
	s.Require().NoError(err)
	s.Require().Len(entries2, 1, "val2 events should not be affected")
}

func (s *KeeperSuite) TestDeleteAllValidatorEvents_NoOpForEmpty() {
	valAddr := sdk.ValAddress([]byte("val_no_events_______"))

	err := s.keeper.DeleteAllValidatorEvents(s.ctx, valAddr)
	s.Require().NoError(err, "should not error on validator with no events")
}
