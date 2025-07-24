package types

const (
	// ModuleName defines the module name
	ModuleName = "supply"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName
)

// VestingAccountsKey for storing vesting account addresses
var VestingAccountsKey = []byte("vestingAccounts")
