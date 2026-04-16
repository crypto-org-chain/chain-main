package types

const (
	// ModuleName defines the module name
	ModuleName = "inflation"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName
)

const (
	ParamsKey = "params"

	// DecayEpochStartKey stores the block height when decay begins (big-endian uint64), set by the upgrade handler or genesis.
	DecayEpochStartKey = "decay_epoch_start"
)
