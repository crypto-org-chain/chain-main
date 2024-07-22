package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the bank contract that must be fulfilled when
// creating a x/supply keeper.
type BankKeeper interface {
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetSupply(ctx context.Context, denom string) sdk.Coin
	LockedCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}

// AccountKeeper defines the account contract that must be fulfilled when
// creating a x/supply keeper.
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	IterateAccounts(ctx context.Context, cb func(account sdk.AccountI) (stop bool))
}
