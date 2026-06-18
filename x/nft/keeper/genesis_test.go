// Cronos.com Chain Copyright 2018-present Cronos.com
package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	nftmodule "github.com/crypto-org-chain/chain-main/v8/x/nft"
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
