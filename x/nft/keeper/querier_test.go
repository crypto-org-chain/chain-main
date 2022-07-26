// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package keeper_test

import (
	"encoding/binary"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/exported"
	keep "github.com/crypto-org-chain/chain-main/v4/x/nft/keeper"
	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

func (suite *KeeperSuite) TestNewQuerier() {
	querier := keep.NewQuerier(suite.keeper, suite.legacyAmino)
	query := abci.RequestQuery{
		Path: "",
		Data: []byte{},
	}
	_, err := querier(suite.ctx, []string{"foo", "bar"}, query)
	suite.Error(err)
}

func (suite *KeeperSuite) TestQuerySupply() {
	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	querier := keep.NewQuerier(suite.keeper, suite.legacyAmino)

	query := abci.RequestQuery{
		Path: "",
		Data: []byte{},
	}

	query.Path = "/custom/nft/supply"
	query.Data = []byte("?")

	res, err := querier(suite.ctx, []string{"supply"}, query)
	suite.Error(err)
	suite.Nil(res)

	queryCollectionParams := types.NewQuerySupplyParams(denomID2, nil)
	bz, errRes := suite.legacyAmino.MarshalJSON(queryCollectionParams)
	suite.Nil(errRes)
	query.Data = bz
	res, err = querier(suite.ctx, []string{"supply"}, query)
	suite.NoError(err)
	supplyResp := binary.LittleEndian.Uint64(res)
	suite.Equal(0, int(supplyResp))

	queryCollectionParams = types.NewQuerySupplyParams(denomID, nil)
	bz, errRes = suite.legacyAmino.MarshalJSON(queryCollectionParams)
	suite.Nil(errRes)
	query.Data = bz

	res, err = querier(suite.ctx, []string{"supply"}, query)
	suite.NoError(err)
	suite.NotNil(res)

	supplyResp = binary.LittleEndian.Uint64(res)
	suite.Equal(1, int(supplyResp))
}

func (suite *KeeperSuite) TestQueryCollection() {
	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	querier := keep.NewQuerier(suite.keeper, suite.legacyAmino)

	query := abci.RequestQuery{
		Path: "",
		Data: []byte{},
	}

	query.Path = "/custom/nft/collection"

	query.Data = []byte("?")
	res, err := querier(suite.ctx, []string{"collection"}, query)
	suite.Error(err)
	suite.Nil(res)

	queryCollectionParams := types.NewQuerySupplyParams(denomID2, nil)
	bz, errRes := suite.legacyAmino.MarshalJSON(queryCollectionParams)
	suite.Nil(errRes)

	query.Data = bz
	_, err = querier(suite.ctx, []string{"collection"}, query)
	suite.NoError(err)

	queryCollectionParams = types.NewQuerySupplyParams(denomID, nil)
	bz, errRes = suite.legacyAmino.MarshalJSON(queryCollectionParams)
	suite.Nil(errRes)

	query.Data = bz
	res, err = querier(suite.ctx, []string{"collection"}, query)
	suite.NoError(err)
	suite.NotNil(res)

	var collection types.Collection
	types.ModuleCdc.MustUnmarshalJSON(res, &collection)
	suite.Len(collection.NFTs, 1)
}

func (suite *KeeperSuite) TestQueryOwner() {
	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	err = suite.keeper.MintNFT(suite.ctx, denomID2, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	querier := keep.NewQuerier(suite.keeper, suite.legacyAmino)
	query := abci.RequestQuery{
		Path: "/custom/nft/owner",
		Data: []byte{},
	}

	query.Data = []byte("?")
	_, err = querier(suite.ctx, []string{"owner"}, query)
	suite.Error(err)

	// query the balance using no denomID so that all denoms will be returns
	params := types.NewQuerySupplyParams("", address)
	bz, err2 := suite.legacyAmino.MarshalJSON(params)
	suite.Nil(err2)
	query.Data = bz

	var out types.Owner
	res, err := querier(suite.ctx, []string{"owner"}, query)
	suite.NoError(err)
	suite.NotNil(res)

	suite.legacyAmino.MustUnmarshalJSON(res, &out)

	// build the owner using both denoms
	idCollection1 := types.NewIDCollection(denomID, []string{tokenID})
	idCollection2 := types.NewIDCollection(denomID2, []string{tokenID})
	owner := types.NewOwner(address, idCollection1, idCollection2)

	suite.EqualValues(out.String(), owner.String())
}

func (suite *KeeperSuite) TestQueryNFT() {
	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	querier := keep.NewQuerier(suite.keeper, suite.legacyAmino)

	query := abci.RequestQuery{
		Path: "",
		Data: []byte{},
	}
	query.Path = "/custom/nft/nft"
	var res []byte

	query.Data = []byte("?")
	res, err = querier(suite.ctx, []string{"nft"}, query)
	suite.Error(err)
	suite.Nil(res)

	params := types.NewQueryNFTParams(denomID2, tokenID2)
	bz, err2 := suite.legacyAmino.MarshalJSON(params)
	suite.Nil(err2)

	query.Data = bz
	res, err = querier(suite.ctx, []string{"nft"}, query)
	suite.Error(err)
	suite.Nil(res)

	params = types.NewQueryNFTParams(denomID, tokenID)
	bz, err2 = suite.legacyAmino.MarshalJSON(params)
	suite.Nil(err2)

	query.Data = bz
	res, err = querier(suite.ctx, []string{"nft"}, query)
	suite.NoError(err)
	suite.NotNil(res)

	var out exported.NFT
	suite.legacyAmino.MustUnmarshalJSON(res, &out)

	suite.Equal(out.GetID(), tokenID)
	suite.Equal(out.GetURI(), tokenURI)
	suite.Equal(out.GetOwner(), address)
}

func (suite *KeeperSuite) TestQueryDenoms() {
	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	err = suite.keeper.MintNFT(suite.ctx, denomID2, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	querier := keep.NewQuerier(suite.keeper, suite.legacyAmino)

	query := abci.RequestQuery{
		Path: "",
		Data: []byte{},
	}
	var res []byte
	query.Path = "/custom/nft/denoms"

	res, err = querier(suite.ctx, []string{"denoms"}, query)
	suite.NoError(err)
	suite.NotNil(res)

	denoms := []string{denomID, denomID2}

	var out []types.Denom
	suite.legacyAmino.MustUnmarshalJSON(res, &out)

	for key, denomInQuestion := range out {
		suite.Equal(denomInQuestion.Id, denoms[key])
	}
}
