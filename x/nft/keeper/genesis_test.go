// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Cronos.org (licensed under the Apache License, Version 2.0)
package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	nftmodule "github.com/crypto-org-chain/chain-main/v8/x/nft"
	"github.com/crypto-org-chain/chain-main/v8/x/nft/types"
)

// Runtime accepts free-form denom names; genesis re-init must too. Regression
// for the bug where ValidateGenesis checked Denom.Name with the strict ID rule.
func (suite *KeeperSuite) TestExportInitGenesisRoundTripFreeFormName() {
	err := suite.keeper.IssueDenom(suite.ctx, "poisoneddenom", "Poisoned Denom Name", schema, "", address)
	suite.NoError(err, "runtime accepts denom names with spaces/uppercase")

	exported := nftmodule.ExportGenesis(suite.ctx, suite.keeper)

	restored := testutil.Setup(isCheckTx, nil)
	restoreCtx := restored.NewContext(isCheckTx)

	suite.NotPanics(func() {
		nftmodule.InitGenesis(restoreCtx, restored.NFTKeeper, *exported)
	}, "exported free-form name must re-init without panic")

	denom, err := restored.NFTKeeper.GetDenom(restoreCtx, "poisoneddenom")
	suite.NoError(err)
	suite.Equal("Poisoned Denom Name", denom.Name)
}

// Legacy IBC NFT transfer bug stored denoms with an empty Name. FixInvalidDenomNames
// rewrites them (Name = Id) so exported genesis re-imports without panicking.
func (suite *KeeperSuite) TestFixInvalidDenomNamesUnbricksExport() {
	const id = "ibcvoucher"
	// SetDenom does not validate Name, so an empty name can reach the store.
	err := suite.keeper.IssueDenom(suite.ctx, id, "", schema, "", address)
	suite.NoError(err)

	// Exported empty-name state must fail validation before the fix.
	suite.Error(types.ValidateGenesis(*nftmodule.ExportGenesis(suite.ctx, suite.keeper)))

	fixed := suite.keeper.FixInvalidDenomNames(suite.ctx)
	suite.Equal(1, fixed)

	denom, err := suite.keeper.GetDenom(suite.ctx, id)
	suite.NoError(err)
	suite.Equal(id, denom.Name, "empty name replaced with id")

	// Name index repaired: old "" entry gone, new entry resolves to the denom.
	suite.False(suite.keeper.HasDenomNm(suite.ctx, ""))
	byName, err := suite.keeper.GetDenomByName(suite.ctx, id)
	suite.NoError(err)
	suite.Equal(id, byName.Id)

	// Export now round-trips cleanly.
	exported := nftmodule.ExportGenesis(suite.ctx, suite.keeper)
	suite.NoError(types.ValidateGenesis(*exported))

	restored := testutil.Setup(isCheckTx, nil)
	restoreCtx := restored.NewContext(isCheckTx)
	suite.NotPanics(func() {
		nftmodule.InitGenesis(restoreCtx, restored.NFTKeeper, *exported)
	})
}
