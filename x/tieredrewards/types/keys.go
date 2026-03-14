package types

import "cosmossdk.io/collections"

var (
	ParamsKey               = collections.NewPrefix(0)
	TiersKey                = collections.NewPrefix(1)
	PositionsKey            = collections.NewPrefix(2)
	NextPositionIdKey       = collections.NewPrefix(3)
	PositionsByOwnerKey     = collections.NewPrefix(4)
	PositionsByTierKey      = collections.NewPrefix(5)
	PositionsByValidatorKey = collections.NewPrefix(6)
	PositionCountByTierKey  = collections.NewPrefix(7)
)

const (
	// ModuleName defines the module name
	ModuleName = "tieredrewards"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RewardsPoolName defines the base reward pool module account name
	RewardsPoolName = "rewards_pool"

	// SecondsPerYear is used to convert durations to years for bonus calculation (365.25 days)
	SecondsPerYear int64 = 31_557_600
)
