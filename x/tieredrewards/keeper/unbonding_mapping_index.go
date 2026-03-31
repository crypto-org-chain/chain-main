package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/indexes"
)

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
