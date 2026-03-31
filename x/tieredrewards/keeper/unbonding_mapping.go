package keeper

import (
	"context"
)

// --- Unbonding (undelegation) mappings ---

// setUnbondingPositionMapping stores unbondingId -> positionId and updates indexes.
func (k Keeper) setUnbondingPositionMapping(ctx context.Context, unbondingId, positionId uint64) error {
	return k.UnbondingDelegationMappings.Set(ctx, unbondingId, positionId)
}

// deleteUnbondingPositionMapping removes unbondingId mapping and its secondary indexes.
func (k Keeper) deleteUnbondingPositionMapping(ctx context.Context, unbondingId uint64) error {
	return k.UnbondingDelegationMappings.Remove(ctx, unbondingId)
}

// deleteUnbondingMappingsForPosition removes every unbondingId mapped to positionId.
func (k Keeper) deleteUnbondingMappingsForPosition(ctx context.Context, positionId uint64) error {
	iter, err := k.UnbondingDelegationMappings.Indexes.ByPosition.MatchExact(ctx, positionId)
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

// stillUnbonding returns true if there are unbonding ids for the position.
func (k Keeper) stillUnbonding(ctx context.Context, positionId uint64) (bool, error) {
	iter, err := k.UnbondingDelegationMappings.Indexes.ByPosition.MatchExact(ctx, positionId)
	if err != nil {
		return false, err
	}
	keys, err := iter.PrimaryKeys()
	if err != nil {
		return false, err
	}
	return len(keys) > 0, nil
}

// --- Redelegation mappings ---

// setRedelegationPositionMapping stores redelegation unbondingId -> positionId.
func (k Keeper) setRedelegationPositionMapping(ctx context.Context, unbondingId, positionId uint64) error {
	return k.RedelegationMappings.Set(ctx, unbondingId, positionId)
}

// deleteRedelegationPositionMapping removes redelegation unbondingId mapping.
func (k Keeper) deleteRedelegationPositionMapping(ctx context.Context, unbondingId uint64) error {
	return k.RedelegationMappings.Remove(ctx, unbondingId)
}

// deleteRedelegationMappingsForPosition removes every redelegation unbondingId mapped to positionId.
func (k Keeper) deleteRedelegationMappingsForPosition(ctx context.Context, positionId uint64) error {
	iter, err := k.RedelegationMappings.Indexes.ByPosition.MatchExact(ctx, positionId)
	if err != nil {
		return err
	}
	ids, err := iter.PrimaryKeys()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := k.deleteRedelegationPositionMapping(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
