package types

import "cosmossdk.io/collections"

var (
	ParamsKey                     = collections.NewPrefix(0)
	TiersKey                      = collections.NewPrefix(1)
	PositionsKey                  = collections.NewPrefix(2)
	NextPositionIdKey             = collections.NewPrefix(3)
	PositionsByOwnerKey           = collections.NewPrefix(4)
	PositionsByTierKey            = collections.NewPrefix(5)
	PositionCountByTierKey        = collections.NewPrefix(6)
	UnbondingIdToPositionIdKey    = collections.NewPrefix(7)
	UnbondingIdsByPositionKey     = collections.NewPrefix(8)
	RedelegationIdToPositionIdKey = collections.NewPrefix(9)
	RedelegationIdsByPositionKey  = collections.NewPrefix(10)
	ValidatorEventsKey            = collections.NewPrefix(11)
	ValidatorEventSeqKey          = collections.NewPrefix(12)
	PositionCountByValidatorKey   = collections.NewPrefix(13)
)

const (
	ModuleName      = "tieredrewards"
	StoreKey        = ModuleName
	RewardsPoolName = "rewards_pool"

	// SecondsPerYear is 365.25 days, used to convert durations to years for bonus calculation.
	SecondsPerYear int64 = 31_557_600
)
