package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

type BankKeeperWrapper struct {
	bankkeeper.Keeper
}

func NewBankKeeperWrapper(base bankkeeper.Keeper) BankKeeperWrapper {
	return BankKeeperWrapper{
		Keeper: base,
	}
}

func (k BankKeeperWrapper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	return k.Keeper.MintCoins(ctx, moduleName, amt)
}
