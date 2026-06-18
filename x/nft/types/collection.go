// Cronos.com Chain Copyright 2018-present Cronos.com
package types

import (
	"github.com/crypto-org-chain/chain-main/v8/x/nft/exported"
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

// NewCollections creates a new NFT Collection
func NewCollections(c ...Collection) []Collection {
	return append([]Collection{}, c...)
}
