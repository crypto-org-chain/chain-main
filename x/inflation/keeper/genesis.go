package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) {
	if err := genState.Validate(); err != nil {
		panic(err)
	}
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&genState.Params)
	if err := store.Set([]byte(types.ParamsKey), bz); err != nil {
		panic(err)
	}
	if genState.DecayEpochStart != 0 {
		if err := k.SetDecayEpochStart(ctx, genState.DecayEpochStart); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis returns the module's exported genesis
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	genesis := types.DefaultGenesis()
	var err error
	genesis.Params, err = k.GetParams(ctx)
	if err != nil {
		panic("fail to get params:" + err.Error())
	}
	epoch, ok, err := k.getDecayEpochStart(ctx)
	if err != nil {
		panic("fail to get decay epoch start:" + err.Error())
	}
	if ok {
		genesis.DecayEpochStart = epoch
	}

	return genesis
}
