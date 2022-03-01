package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v4/x/supply/types"
)

// InitGenesis initializes the supply module's state from a given genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	k.SetVestingAccounts(ctx, k.FetchVestingAccounts(ctx))
}

// ExportGenesis returns the supplu module's genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return types.DefaultGenesis()
}
