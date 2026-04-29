// Package v8 is a self-contained quarantine of tieredrewards store key
// prefixes as they existed at the v8 upgrade boundary. The v8 upgrade handler
// iterates each prefix and purges its keys, wiping all pre-v8 module state
// before the new per-position-delegator codebase takes over.
//
// This package MUST NOT import from x/tieredrewards/types or x/tieredrewards/keeper.
// Changes to the live types/keys.go after the upgrade ships must not alter the
// bytes below — they are a frozen record of the on-chain layout at upgrade
// time. Once testnet has upgraded past v8, this package can be deleted.
package v8

// Store key prefixes — byte values copied from types/keys.go as of the
// pre-v8 release. See the collections.NewPrefix(n) calls that produced them.
var (
	ParamsKeyPrefix                     = []byte{0}
	TiersKeyPrefix                      = []byte{1}
	PositionsKeyPrefix                  = []byte{2}
	NextPositionIdKeyPrefix             = []byte{3}
	PositionsByOwnerKeyPrefix           = []byte{4}
	PositionsByTierKeyPrefix            = []byte{5}
	PositionsByValidatorKeyPrefix       = []byte{6}
	PositionCountByTierKeyPrefix        = []byte{7}
	ValidatorRewardRatioKeyPrefix       = []byte{8}
	UnbondingIdToPositionIdKeyPrefix    = []byte{9}
	UnbondingIdsByPositionKeyPrefix     = []byte{10}
	RedelegationIdToPositionIdKeyPrefix = []byte{11}
	RedelegationIdsByPositionKeyPrefix  = []byte{12}
	ValidatorEventsKeyPrefix            = []byte{13}
	ValidatorEventSeqKeyPrefix          = []byte{14}
	PositionCountByValidatorKeyPrefix   = []byte{15}
)

// AllPrefixes returns every prefix the v8 upgrade handler must wipe.
// Returned in the order they should be processed.
func AllPrefixes() [][]byte {
	return [][]byte{
		ParamsKeyPrefix,
		TiersKeyPrefix,
		PositionsKeyPrefix,
		NextPositionIdKeyPrefix,
		PositionsByOwnerKeyPrefix,
		PositionsByTierKeyPrefix,
		PositionsByValidatorKeyPrefix,
		PositionCountByTierKeyPrefix,
		ValidatorRewardRatioKeyPrefix,
		UnbondingIdToPositionIdKeyPrefix,
		UnbondingIdsByPositionKeyPrefix,
		RedelegationIdToPositionIdKeyPrefix,
		RedelegationIdsByPositionKeyPrefix,
		ValidatorEventsKeyPrefix,
		ValidatorEventSeqKeyPrefix,
		PositionCountByValidatorKeyPrefix,
	}
}
