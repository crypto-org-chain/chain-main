package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidatorEventEntry pairs a sequence number with a ValidatorEvent,
// used when iterating events for a validator.
type ValidatorEventEntry struct {
	Seq   uint64
	Event types.ValidatorEvent
}

// appendValidatorEvent auto-increments the event sequence for the validator
// and stores the event. Returns the assigned sequence number.
func (k Keeper) appendValidatorEvent(ctx context.Context, valAddr sdk.ValAddress, event types.ValidatorEvent) (uint64, error) {
	seq, err := k.nextValidatorEventSeq(ctx, valAddr)
	if err != nil {
		return 0, err
	}

	if err := k.ValidatorEvents.Set(ctx, collections.Join(valAddr, seq), event); err != nil {
		return 0, err
	}

	return seq, nil
}

// nextValidatorEventSeq returns the current next-seq for a validator
// and increments it atomically. First call returns 1.
func (k Keeper) nextValidatorEventSeq(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	current, err := k.ValidatorEventNextSeq.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		current = 1 // first event gets seq 1
	} else if err != nil {
		return 0, err
	}

	seq := current
	if err := k.ValidatorEventNextSeq.Set(ctx, valAddr, seq+1); err != nil {
		return 0, err
	}

	return seq, nil
}

// getValidatorEventNextSeq returns the next-seq for a validator without incrementing it.
// Returns 1 if no events have ever been appended (first event would get seq 1).
func (k Keeper) getValidatorEventNextSeq(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	seq, err := k.ValidatorEventNextSeq.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		return 1, nil
	}
	return seq, err
}

// getValidatorEventLatestSeq returns the sequence number of the most recent
// event for a validator. Returns 0 if no events have been appended.
// Used when creating positions to set LastEventSeq so that only future events
// are processed.
func (k Keeper) getValidatorEventLatestSeq(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	nextSeq, err := k.getValidatorEventNextSeq(ctx, valAddr)
	if err != nil {
		return 0, err
	}
	return nextSeq - 1, nil
}

// getValidatorEventsSince returns all events for a validator with sequence > startSeq,
// in ascending order.
func (k Keeper) getValidatorEventsSince(ctx context.Context, valAddr sdk.ValAddress, startSeq uint64) ([]ValidatorEventEntry, error) {
	// Range from (valAddr, startSeq+1) to end of valAddr prefix.
	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr).
		StartExclusive(startSeq)

	var entries []ValidatorEventEntry
	err := k.ValidatorEvents.Walk(ctx, rng, func(key collections.Pair[sdk.ValAddress, uint64], event types.ValidatorEvent) (bool, error) {
		entries = append(entries, ValidatorEventEntry{Seq: key.K2(), Event: event})
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

// deleteAllValidatorEvents removes all events and the next-seq entry
// for a validator. Used during validator removal cleanup.
func (k Keeper) deleteAllValidatorEvents(ctx context.Context, valAddr sdk.ValAddress) error {
	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr)
	err := k.ValidatorEvents.Walk(ctx, rng, func(key collections.Pair[sdk.ValAddress, uint64], _ types.ValidatorEvent) (bool, error) {
		return false, k.ValidatorEvents.Remove(ctx, key)
	})
	if err != nil {
		return err
	}

	has, err := k.ValidatorEventNextSeq.Has(ctx, valAddr)
	if err != nil {
		return err
	}
	if has {
		return k.ValidatorEventNextSeq.Remove(ctx, valAddr)
	}
	return nil
}
