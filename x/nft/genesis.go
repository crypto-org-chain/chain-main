// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package nft

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/keeper"
	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

// InitGenesis stores the NFT genesis.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, data types.GenesisState) {
	if err := types.ValidateGenesis(data); err != nil {
		panic(err.Error())
	}

	for _, c := range data.Collections {
		if err := k.SetDenom(ctx, c.Denom); err != nil {
			panic(err)
		}
		if err := k.SetGenesisCollection(ctx, c); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	return types.NewGenesisState(k.GetCollections(ctx))
}

// DefaultGenesisState returns a default genesis state
func DefaultGenesisState() *types.GenesisState {
	return types.NewGenesisState([]types.Collection{})
}
