package keeper

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-com/chain-main/x/chainmain/types"
)

type (
	// Keeper of the chainmain store
	Keeper struct {
		cdc      codec.BinaryMarshaler
		storeKey sdk.StoreKey
		memKey   sdk.StoreKey
	}
)

// NewKeeper creates a chainmain keeper
func NewKeeper(cdc codec.BinaryMarshaler, storeKey, memKey sdk.StoreKey) *Keeper {
	return &Keeper{
		cdc:      cdc,
		storeKey: storeKey,
		memKey:   memKey,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
