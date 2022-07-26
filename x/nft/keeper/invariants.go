// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package keeper

// DONTCOVER

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

// RegisterInvariants registers all supply invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "supply", SupplyInvariant(k))
}

// AllInvariants runs all invariants of the NFT module.
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		return SupplyInvariant(k)(ctx)
	}
}

// SupplyInvariant checks that the total amount of NFTs on collections matches the total amount owned by addresses
func SupplyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		ownersCollectionsSupply := make(map[string]uint64)
		var msg string
		count := 0

		owners, err := k.GetOwners(ctx)

		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "supply", "unable to fetch owners"), true
		}

		for _, owner := range owners {
			for _, idCollection := range owner.IDCollections {
				ownersCollectionsSupply[idCollection.DenomId] += uint64(idCollection.Supply())
			}
		}

		for denom, supply := range ownersCollectionsSupply {
			if supply != k.GetTotalSupply(ctx, denom) {
				count++
				msg += fmt.Sprintf(
					"total %s NFTs supply invariance:\n"+
						"\ttotal %s NFTs supply: %d\n"+
						"\tsum of %s NFTs by owner: %d\n",
					denom, denom, supply, denom, ownersCollectionsSupply[denom],
				)
			}
		}
		broken := count != 0

		return sdk.FormatInvariant(
			types.ModuleName, "supply",
			fmt.Sprintf("%d NFT supply invariants found\n%s", count, msg),
		), broken
	}
}
