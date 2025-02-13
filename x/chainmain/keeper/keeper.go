package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/log"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v4/x/chainmain/types"
)

type (
	// Keeper of the chainmain store
	Keeper struct {
		cdc      codec.BinaryCodec
		storeKey storetypes.StoreKey
		memKey   storetypes.StoreKey
	}
)

// NewKeeper creates a chainmain keeper
func NewKeeper(cdc codec.BinaryCodec, storeKey, memKey storetypes.StoreKey) *Keeper {
	return &Keeper{
		cdc:      cdc,
		storeKey: storeKey,
		memKey:   memKey,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
