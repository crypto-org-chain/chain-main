package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
)

// setUnbondingPositionMapping stores unbondingId -> positionId and updates indexes.
func (k Keeper) setUnbondingPositionMapping(ctx context.Context, unbondingId, positionId uint64) error {
	return k.UnbondingMappings.Set(ctx, unbondingId, positionId)
}

// deleteUnbondingPositionMapping removes unbondingId mapping and its secondary indexes.
func (k Keeper) deleteUnbondingPositionMapping(ctx context.Context, unbondingId uint64) error {
	if err := k.UnbondingMappings.Remove(ctx, unbondingId); err != nil && !errors.Is(err, collections.ErrNotFound) {
		return err
	}
	return nil
}

// deleteUnbondingMappingsForPosition removes every unbondingId mapped to positionId.
// Lookups are index-based via positionId -> unbondingId[].
func (k Keeper) deleteUnbondingMappingsForPosition(ctx context.Context, positionId uint64) error {
	iter, err := k.UnbondingMappings.Indexes.ByPosition.MatchExact(ctx, positionId)
	if err != nil {
		return err
	}
	unbondingIDs, err := iter.PrimaryKeys()
	if err != nil {
		return err
	}
	for _, unbondingID := range unbondingIDs {
		if err := k.deleteUnbondingPositionMapping(ctx, unbondingID); err != nil {
			return err
		}
	}

	return nil
}
