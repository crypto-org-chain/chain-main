package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/maxsupply/types"

	sdkmath "cosmossdk.io/math"
)

func (suite *KeeperSuite) TestMaxSupply() {
	// Set up expected max supply value
	expectedMaxSupply := sdkmath.NewInt(1000000000)
	params := types.DefaultParams()
	params.MaxSupply = expectedMaxSupply
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	response, err := suite.queryClient.MaxSupply(suite.ctx, &types.QueryMaxSupplyRequest{})
	suite.NoError(err)

	suite.Equal(response.MaxSupply, expectedMaxSupply.String())
}

func (suite *KeeperSuite) TestBurnedAddresses() {
	// Set up expected burned addresses
	expectedBurnedAddresses := []string{
		"cro1abc123def456ghi789jkl012mno345pqr678stu",
		"cro1def456ghi789jkl012mno345pqr678stu901vwx",
		"cro1ghi789jkl012mno345pqr678stu901vwx234yz0",
	}

	params := types.DefaultParams()
	params.BurnedAddresses = expectedBurnedAddresses
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	response, err := suite.queryClient.BurnedAddresses(suite.ctx, &types.QueryBurnedAddressesRequest{})
	suite.NoError(err)

	suite.Equal(len(response.BurnedAddresses), len(expectedBurnedAddresses))
	suite.Equal(response.BurnedAddresses, expectedBurnedAddresses)
}

func (suite *KeeperSuite) TestBurnedAddressesEmpty() {
	// Test with empty burned addresses list
	params := types.DefaultParams()
	params.BurnedAddresses = []string{}
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	response, err := suite.queryClient.BurnedAddresses(suite.ctx, &types.QueryBurnedAddressesRequest{})
	suite.NoError(err)

	suite.Equal(len(response.BurnedAddresses), 0)
	suite.Empty(response.BurnedAddresses)
}
