package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"

	sdkmath "cosmossdk.io/math"
)

func (suite *KeeperSuite) TestMaxSupply() {
	// Set up expected max supply value
	expectedMaxSupply := sdkmath.NewInt(1000000000)
	params := types.DefaultParams()
	params.MaxSupply = expectedMaxSupply
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	response, err := suite.queryClient.Params(suite.ctx, &types.QueryParamsRequest{})
	suite.NoError(err)

	suite.Equal(response.Params.MaxSupply.String(), expectedMaxSupply.String())
}

func (suite *KeeperSuite) TestBurnedAddresses() {
	// Set up expected burned addresses
	expectedBurnedAddresses := []string{
		"cosmos1dej28rxfh39axghzlcusd98qhpkdarcqqu23ua",
		"cosmos1g69pjvgvdug5m9kphwh284rvls4g5jnrg4p8dm",
		"cosmos1zpdh03ej2h9ct3lgjydqp3upqkktq322dcvwjm",
	}

	params := types.DefaultParams()
	params.BurnedAddresses = expectedBurnedAddresses
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	response, err := suite.queryClient.Params(suite.ctx, &types.QueryParamsRequest{})
	suite.NoError(err)

	burnedAddresses := response.Params.BurnedAddresses

	suite.Equal(len(burnedAddresses), len(expectedBurnedAddresses))
	suite.Equal(burnedAddresses, expectedBurnedAddresses)
}

func (suite *KeeperSuite) TestBurnedAddressesEmpty() {
	// Test with empty burned addresses list
	params := types.DefaultParams()
	params.BurnedAddresses = []string{}
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	response, err := suite.queryClient.Params(suite.ctx, &types.QueryParamsRequest{})
	suite.NoError(err)

	burnedAddresses := response.Params.BurnedAddresses

	suite.Equal(len(burnedAddresses), 0)
	suite.Empty(burnedAddresses)
}
