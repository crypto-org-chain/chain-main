// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Cronos.org (licensed under the Apache License, Version 2.0)
package keeper_test

func (suite *KeeperSuite) TestGetOwners() {
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	err = suite.keeper.MintNFT(suite.ctx, denomID, tokenID2, tokenNm2, tokenURI, tokenData, address, address2)
	suite.NoError(err)

	err = suite.keeper.MintNFT(suite.ctx, denomID, tokenID3, tokenNm3, tokenURI, tokenData, address, address3)
	suite.NoError(err)

	owners, err := suite.keeper.GetOwners(suite.ctx)
	suite.NoError(err)
	suite.Equal(3, len(owners))

	err = suite.keeper.MintNFT(suite.ctx, denomID2, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	err = suite.keeper.MintNFT(suite.ctx, denomID2, tokenID2, tokenNm2, tokenURI, tokenData, address, address2)
	suite.NoError(err)

	err = suite.keeper.MintNFT(suite.ctx, denomID2, tokenID3, tokenNm3, tokenURI, tokenData, address, address3)
	suite.NoError(err)

	owners, err = suite.keeper.GetOwners(suite.ctx)
	suite.NoError(err)
	suite.Equal(3, len(owners))
}
