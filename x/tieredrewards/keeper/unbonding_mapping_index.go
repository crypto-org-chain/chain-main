package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/indexes"
)

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

type UnbondingMappingsIndexes struct {
	ByPosition *indexes.Multi[uint64, uint64, uint64]
}

func (i UnbondingMappingsIndexes) IndexesList() []collections.Index[uint64, uint64] {
	return []collections.Index[uint64, uint64]{i.ByPosition}
}
