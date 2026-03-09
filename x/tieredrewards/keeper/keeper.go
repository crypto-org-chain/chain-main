package keeper

import (
	"context"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// Keeper of the tieredrewards store.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService storetypes.KVStoreService
	authority    string

	Schema collections.Schema
	Params collections.Item[types.Params]
}

// NewKeeper creates a new tieredrewards Keeper instance.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService storetypes.KVStoreService,
	authority string,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
		Params:       collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the x/tieredrewards module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}
