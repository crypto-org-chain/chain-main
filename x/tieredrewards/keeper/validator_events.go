package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EventEntry pairs a sequence number with a ValidatorEvent,
// used when iterating events for a validator.
type EventEntry struct {
	Seq   uint64
	Event types.ValidatorEvent
}

// appendValidatorEvent auto-increments the event sequence for the validator
// and stores the event. Returns the assigned sequence number.
func (k Keeper) appendValidatorEvent(ctx context.Context, valAddr sdk.ValAddress, event types.ValidatorEvent) (uint64, error) {
	seq, err := k.incrementValidatorEventSeq(ctx, valAddr)
	if err != nil {
		return 0, err
	}

	if err := k.ValidatorEvents.Set(ctx, collections.Join(valAddr, seq), event); err != nil {
		return 0, err
	}

	return seq, nil
}

// incrementValidatorEventSeq reads the current (last used) seq for a validator,
// increments it, persists the new value, and returns the new seq.
// First call returns 1.
func (k Keeper) incrementValidatorEventSeq(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	current, err := k.ValidatorEventSeq.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		current = 0
	} else if err != nil {
		return 0, err
	}

	next := current + 1
	if err := k.ValidatorEventSeq.Set(ctx, valAddr, next); err != nil {
		return 0, err
	}

	return next, nil
}

// getValidatorEventLatestSeq returns the sequence number of the most recent
// event for a validator. Returns 0 if no events have been appended.
// Used when creating positions to set LastEventSeq so that only future events
// are processed.
func (k Keeper) getValidatorEventLatestSeq(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	seq, err := k.ValidatorEventSeq.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		return 0, nil
	}
	return seq, err
}

// getValidatorEventsSince returns all events for a validator with sequence > startSeq,
// in ascending order.
func (k Keeper) getValidatorEventsSince(ctx context.Context, valAddr sdk.ValAddress, startSeq uint64) ([]EventEntry, error) {
	// Range from (valAddr, startSeq+1) to end of valAddr prefix.
	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr).
		StartExclusive(startSeq)

	var entries []EventEntry
	err := k.ValidatorEvents.Walk(ctx, rng, func(key collections.Pair[sdk.ValAddress, uint64], event types.ValidatorEvent) (bool, error) {
		entries = append(entries, EventEntry{Seq: key.K2(), Event: event})
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// decrementEventRefCount decrements the reference count of a validator event.
// If the reference count reaches zero, the event is deleted.
func (k Keeper) decrementEventRefCount(ctx context.Context, valAddr sdk.ValAddress, seq uint64) error {
	key := collections.Join(valAddr, seq)
	event, err := k.ValidatorEvents.Get(ctx, key)
	if errors.Is(err, collections.ErrNotFound) {
		return nil // already cleaned up
	}
	if err != nil {
		return err
	}

	if event.ReferenceCount <= 1 {
		return k.ValidatorEvents.Remove(ctx, key)
	}

	event.ReferenceCount--
	return k.ValidatorEvents.Set(ctx, key, event)
}

// deleteValidatorEventSeq removes the current-seq entry
// for a validator. Used during validator removal cleanup.
func (k Keeper) deleteValidatorEventSeq(ctx context.Context, valAddr sdk.ValAddress) error {
	has, err := k.ValidatorEventSeq.Has(ctx, valAddr)
	if err != nil {
		return err
	}
	if has {
		return k.ValidatorEventSeq.Remove(ctx, valAddr)
	}
	return nil
}

func (k Keeper) hasValidatorEvents(ctx context.Context, valAddr sdk.ValAddress) (bool, error) {
	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr)
	found := false
	err := k.ValidatorEvents.Walk(ctx, rng, func(_ collections.Pair[sdk.ValAddress, uint64], _ types.ValidatorEvent) (bool, error) {
		found = true
		return true, nil
	})
	return found, err
}
