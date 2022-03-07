package keeper_test

import (
	"fmt"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

func (suite *KeeperSuite) TestIsCreator() {
	// denom should exist already (see `SetupTest()` of `KeeperSuite`)
	denom, err := suite.keeper.GetDenom(suite.ctx, denomID)
	suite.NoError(err)
	suite.Equal(denom.Creator, address.String())

	_, err = suite.keeper.IsDenomCreator(suite.ctx, "nonExistentDenom", address)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("not found denomID: nonExistentDenom: %s", types.ErrInvalidDenom))
	}

	_, err = suite.keeper.IsDenomCreator(suite.ctx, denomID, address2)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("%s is not the creator of %s: %s", address2, denomID, types.ErrUnauthorized))
	}

	_, err = suite.keeper.IsDenomCreator(suite.ctx, denomID, address)
	suite.NoError(err)
}
