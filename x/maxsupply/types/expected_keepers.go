package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BankKeeper interface {
	GetSupply(ctx context.Context, denom string) sdk.Coin
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

type StakingKeeper interface {
	BondDenom(ctx context.Context) (string, error)
}
