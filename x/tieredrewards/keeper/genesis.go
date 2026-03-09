package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState) {
	if err := k.Params.Set(ctx, data.Params); err != nil {
		panic(err)
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.Params.Get(ctx)
	if err != nil {
		panic(err)
	}

	return &types.GenesisState{Params: params}
}
