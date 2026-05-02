// Package v7testnet is a self-contained quarantine of tieredrewards store
// key prefixes as they existed at the v7.1.0-testnet upgrade boundary. The
// v7.1.0-testnet upgrade handler iterates each prefix and purges its keys,
// wiping pre-rewrite testnet module state before the new per-position-
// delegator codebase takes over.
//
// This package MUST NOT import from x/tieredrewards/types or x/tieredrewards/keeper.
// Changes to the live types/keys.go after the upgrade ships must not alter the
// bytes below — they are a frozen record of the on-chain layout at upgrade
// time. Once testnet has upgraded past v7.1.0-testnet, this package can be
// deleted.
package v7testnet

// Store key prefixes — byte values copied from types/keys.go as of the
// pre-rewrite release. See the collections.NewPrefix(n) calls that produced them.
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

// AllPrefixes returns every pre-rewrite prefix this module ever owned. Used
// as a reference / documentation of the full store footprint.
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

// StateToPurge returns the prefixes the v7.1.0-testnet upgrade handler should
// wipe. Excludes Params and Tiers so that operator-configured values survive
// the upgrade: the Params / Tier proto shapes are unchanged, so their stored
// bytes decode cleanly under the new code. Everything else (positions,
// secondary indexes, mappings, validator events, counters, and the retired
// ValidatorRewardRatio collection) is lifecycle state tied to the pre-rewrite
// shared-pool delegator model and cannot be carried over.
func StateToPurge() [][]byte {
	return [][]byte{
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
