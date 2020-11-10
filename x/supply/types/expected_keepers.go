package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/bank/exported"
)

// BankKeeper defines the bank contract that must be fulfilled when
// creating a x/supply keeper.
type BankKeeper interface {
	GetAllBalances(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	GetSupply(ctx sdk.Context) exported.SupplyI
	LockedCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
}

// AccountKeeper defines the account contract that must be fulfilled when
// creating a x/supply keeper.
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	IterateAccounts(sdk.Context, func(types.AccountI) bool)
}
