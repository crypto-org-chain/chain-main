// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package types

import (
	"github.com/crypto-org-chain/chain-main/v4/x/nft/exported"
)

// NewCollection creates a new NFT Collection
func NewCollection(denom Denom, nfts []exported.NFT) (c Collection) {
	c.Denom = denom
	for _, nft := range nfts {
		c = c.AddNFT(nft.(BaseNFT))
	}
	return c
}

// AddNFT adds an NFT to the collection
func (c Collection) AddNFT(nft BaseNFT) Collection {
	c.NFTs = append(c.NFTs, nft)
	return c
}

func (c Collection) Supply() int {
	return len(c.NFTs)
}

// NewCollection creates a new NFT Collection
func NewCollections(c ...Collection) []Collection {
	return append([]Collection{}, c...)
}
