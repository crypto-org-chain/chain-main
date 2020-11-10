package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-com/chain-main/x/supply/types"
)

// InitGenesis initializes the supply module's state from a given genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	k.SetVestingAccounts(ctx, genState.GetVestingAccounts())
}

// ExportGenesis returns the supplu module's genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	vestingAccounts := k.GetVestingAccounts(ctx)
	return types.NewGenesisState(vestingAccounts)
}
