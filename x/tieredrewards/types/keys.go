package types

import "cosmossdk.io/collections"

var (
	ParamsKey                     = collections.NewPrefix(0)
	TiersKey                      = collections.NewPrefix(1)
	PositionsKey                  = collections.NewPrefix(2)
	NextPositionIdKey             = collections.NewPrefix(3)
	PositionsByOwnerKey           = collections.NewPrefix(4)
	PositionsByTierKey            = collections.NewPrefix(5)
	PositionsByValidatorKey       = collections.NewPrefix(6)
	PositionCountByTierKey        = collections.NewPrefix(7)
	UnbondingIdToPositionIdKey    = collections.NewPrefix(8)
	UnbondingIdsByPositionKey     = collections.NewPrefix(9)
	RedelegationIdToPositionIdKey = collections.NewPrefix(10)
	RedelegationIdsByPositionKey  = collections.NewPrefix(11)
	ValidatorEventsKey            = collections.NewPrefix(12)
	ValidatorEventSeqKey          = collections.NewPrefix(13)
	PositionCountByValidatorKey   = collections.NewPrefix(14)
)

const (
	ModuleName      = "tieredrewards"
	StoreKey        = ModuleName
	RewardsPoolName = "rewards_pool"

	// SecondsPerYear is 365.25 days, used to convert durations to years for bonus calculation.
	SecondsPerYear int64 = 31_557_600
)
