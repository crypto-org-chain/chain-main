// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package keeper_test

import (
	gocontext "context"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

func (suite *KeeperSuite) TestSupply() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	response, err := suite.queryClient.Supply(gocontext.Background(), &types.QuerySupplyRequest{
		DenomId: denomID,
		Owner:   address.String(),
	})

	suite.NoError(err)
	suite.Equal(1, int(response.Amount))
}

func (suite *KeeperSuite) TestOwner() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	response, err := suite.queryClient.Owner(gocontext.Background(), &types.QueryOwnerRequest{
		DenomId: denomID,
		Owner:   address.String(),
	})

	suite.NoError(err)
	suite.NotNil(response.Owner)
	suite.Contains(response.Owner.IDCollections[0].TokenIds, tokenID)
}

func (suite *KeeperSuite) TestCollection() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	response, err := suite.queryClient.Collection(gocontext.Background(), &types.QueryCollectionRequest{
		DenomId: denomID,
	})

	suite.NoError(err)
	suite.NotNil(response.Collection)
	suite.Len(response.Collection.NFTs, 1)
	suite.Equal(response.Collection.NFTs[0].Id, tokenID)
}

func (suite *KeeperSuite) TestDenom() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	response, err := suite.queryClient.Denom(gocontext.Background(), &types.QueryDenomRequest{
		DenomId: denomID,
	})

	suite.NoError(err)
	suite.NotNil(response.Denom)
	suite.Equal(response.Denom.Id, denomID)
}

func (suite *KeeperSuite) TestDenoms() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	response, err := suite.queryClient.Denoms(gocontext.Background(), &types.QueryDenomsRequest{})

	suite.NoError(err)
	suite.NotEmpty(response.Denoms)
	suite.Equal(response.Denoms[0].Id, denomID)
}

func (suite *KeeperSuite) TestNFT() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	response, err := suite.queryClient.NFT(gocontext.Background(), &types.QueryNFTRequest{
		DenomId: denomID,
		TokenId: tokenID,
	})

	suite.NoError(err)
	suite.NotEmpty(response.NFT)
	suite.Equal(response.NFT.Id, tokenID)
}
