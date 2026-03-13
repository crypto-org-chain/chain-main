package types

import "cosmossdk.io/collections"

var ParamsKey = collections.NewPrefix(0)

const (
	// ModuleName defines the module name
	ModuleName = "tieredrewards"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RewardsPoolName defines the base reward pool module account name
	RewardsPoolName = "rewards_pool"

	// TierPoolName is a separate pool for bonus payouts
	TierPoolName = "tier_reward_pool"

	// SecondsPerYear is a chain constant for bonus APY calculations (365.25 days).
	SecondsPerYear = int64(31_557_600)
)

// Collection prefixes for state storage
var (
	// ParamsKey is already defined above (prefix 0)
	PositionByIDPrefix     = collections.NewPrefix(1)
	PositionsByOwnerPrefix = collections.NewPrefix(2)
	NextPositionIDKey      = collections.NewPrefix(3)
	TotalTierSharesPrefix  = collections.NewPrefix(5)
	PositionsByValidatorPrefix = collections.NewPrefix(6)
	UnbondingPositionsPrefix   = collections.NewPrefix(7)
)
