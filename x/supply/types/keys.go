package types

const (
	// ModuleName defines the module name
	ModuleName = "supply"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName
)

// VestingAccountsKey for storing vesting account addresses
var VestingAccountsKey = []byte("vestingAccounts")
