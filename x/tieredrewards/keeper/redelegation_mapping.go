package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/indexes"
)

// RedelegationMappingsIndexes defines secondary indexes on RedelegationMappings.
// ByPosition is a position_id -> unbonding_id multi-index.
// Used for deleting position redelegation mappings based on position_id.
type RedelegationMappingsIndexes struct {
	ByPosition *indexes.Multi[uint64, uint64, uint64]
}

func (i RedelegationMappingsIndexes) IndexesList() []collections.Index[uint64, uint64] {
	return []collections.Index[uint64, uint64]{i.ByPosition}
}

func newRedelegationMappingsIndexes(sb *collections.SchemaBuilder) RedelegationMappingsIndexes {
	return RedelegationMappingsIndexes{
		ByPosition: indexes.NewMulti(
			sb,
			types.RedelegationMappingsByPositionKey,
			"redelegation_mappings_by_position",
			collections.Uint64Key,
			collections.Uint64Key,
			func(_, positionId uint64) (uint64, error) {
				return positionId, nil
			},
		),
	}
}

func (k Keeper) setRedelegationMapping(ctx context.Context, unbondingId, positionId uint64) error {
	return k.RedelegationMappings.Set(ctx, unbondingId, positionId)
}

func (k Keeper) getRedelegationMapping(ctx context.Context, unbondingId uint64) (uint64, error) {
	return k.RedelegationMappings.Get(ctx, unbondingId)
}

func (k Keeper) deleteRedelegationMapping(ctx context.Context, unbondingId uint64) error {
	return k.RedelegationMappings.Remove(ctx, unbondingId)
}

func (k Keeper) deletePositionRedelegationMappings(ctx context.Context, positionId uint64) error {
	iter, err := k.RedelegationMappings.Indexes.ByPosition.MatchExact(ctx, positionId)
	if err != nil {
		return err
	}
	defer iter.Close()

	unbondingIds, err := iter.PrimaryKeys()
	if err != nil {
		return err
	}
	for _, unbondingId := range unbondingIds {
		if err := k.RedelegationMappings.Remove(ctx, unbondingId); err != nil {
			if errors.Is(err, collections.ErrNotFound) {
				continue
			}
			return err
		}
	}
	return nil
}
