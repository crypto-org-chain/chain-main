package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/indexes"
)

// UnbondingMappingsIndexes defines the secondary indexes for unbonding mappings.
type UnbondingMappingsIndexes struct {
	ByPosition *indexes.Multi[uint64, uint64, uint64]
}

func (i UnbondingMappingsIndexes) IndexesList() []collections.Index[uint64, uint64] {
	return []collections.Index[uint64, uint64]{i.ByPosition}
}

func newUnbondingMappingsIndexes(sb *collections.SchemaBuilder) UnbondingMappingsIndexes {
	return UnbondingMappingsIndexes{
		ByPosition: indexes.NewMulti(
			sb,
			types.UnbondingIdsByPositionKey,
			"unbonding_ids_by_position",
			collections.Uint64Key,
			collections.Uint64Key,
			func(_, positionID uint64) (uint64, error) {
				return positionID, nil
			},
		),
	}
}

func newRedelegationMappingsIndexes(sb *collections.SchemaBuilder) UnbondingMappingsIndexes {
	return UnbondingMappingsIndexes{
		ByPosition: indexes.NewMulti(
			sb,
			types.RedelegationIdsByPositionKey,
			"redelegation_ids_by_position",
			collections.Uint64Key,
			collections.Uint64Key,
			func(_, positionID uint64) (uint64, error) {
				return positionID, nil
			},
		),
	}
}

func (k Keeper) hasPositionMapping(
	ctx context.Context,
	mappings *collections.IndexedMap[uint64, uint64, UnbondingMappingsIndexes],
	positionId uint64,
) (bool, error) {
	iter, err := mappings.Indexes.ByPosition.MatchExact(ctx, positionId)
	if err != nil {
		return false, err
	}
	keys, err := iter.PrimaryKeys()
	if err != nil {
		return false, err
	}
	return len(keys) > 0, nil
}

// --- Unbonding (undelegation) Mappings ---

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

// stillUnbonding returns true if there are pending undelegation unbonding ids for the position.
func (k Keeper) stillUnbonding(ctx context.Context, positionId uint64) (bool, error) {
	return k.hasPositionMapping(ctx, k.UnbondingDelegationMappings, positionId)
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

// stillRedelegating returns true if there are pending redelegation ids for the position.
func (k Keeper) stillRedelegating(ctx context.Context, positionId uint64) (bool, error) {
	return k.hasPositionMapping(ctx, k.RedelegationMappings, positionId)
}
