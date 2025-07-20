package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v4/x/maxsupply/types"

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
