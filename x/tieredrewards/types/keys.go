package types

import "cosmossdk.io/collections"

var (
	ParamsKey = collections.NewPrefix(0)
)

const (
	// ModuleName defines the module name
	ModuleName = "tieredrewards"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// BaseRewardsPoolName defines the base reward pool module account name
	BaseRewardsPoolName = "base_rewards_pool"
)
