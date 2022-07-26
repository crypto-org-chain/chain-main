// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package simulation

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

const (
	kitties = "kitties"
	doggos  = "doggos"
)

// RandomizedGenState generates a random GenesisState for nft
func RandomizedGenState(simState *module.SimulationState) {
	collections := types.NewCollections(
		types.NewCollection(
			types.Denom{
				Id:      doggos,
				Name:    doggos,
				Schema:  "",
				Creator: "",
			},
			types.NFTs{},
		),
		types.NewCollection(
			types.Denom{
				Id:      kitties,
				Name:    kitties,
				Schema:  "",
				Creator: "",
			},
			types.NFTs{}),
	)
	for _, acc := range simState.Accounts {
		// 10% of accounts own an NFT
		if simState.Rand.Intn(100) < 10 {
			baseNFT := types.NewBaseNFT(
				RandnNFTID(simState.Rand, types.MinDenomLen, types.MaxDenomLen), // id
				simtypes.RandStringOfLength(simState.Rand, 10),
				acc.Address,
				simtypes.RandStringOfLength(simState.Rand, 45), // tokenURI
				simtypes.RandStringOfLength(simState.Rand, 10),
			)

			// 50% doggos and 50% kitties
			if simState.Rand.Intn(100) < 50 {
				collections[0].Denom.Creator = baseNFT.Owner
				collections[0] = collections[0].AddNFT(baseNFT)
			} else {
				collections[1].Denom.Creator = baseNFT.Owner
				collections[1] = collections[1].AddNFT(baseNFT)
			}
		}
	}

	nftGenesis := types.NewGenesisState(collections)

	bz, err := json.MarshalIndent(nftGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated %s parameters:\n%s\n", types.ModuleName, bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(nftGenesis)
}

func RandnNFTID(r *rand.Rand, min, max int) string {
	n := simtypes.RandIntBetween(r, min, max)
	id := simtypes.RandStringOfLength(r, n)
	return strings.ToLower(id)
}
