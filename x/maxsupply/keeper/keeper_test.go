package keeper_test

import (
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/maxsupply/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/maxsupply/types"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var isCheckTx = false

// (Amino is still needed for Ledger at the moment)
type KeeperSuite struct {
	suite.Suite

	legacyAmino *codec.LegacyAmino
	ctx         sdk.Context
	keeper      keeper.Keeper
	app         *app.ChainApp

	queryClient types.QueryClient
}

func (suite *KeeperSuite) SetupTest() {
	a := testutil.Setup(isCheckTx, nil)
	suite.app = a
	suite.legacyAmino = a.LegacyAmino()
	suite.ctx = a.BaseApp.NewContext(isCheckTx).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	suite.keeper = a.MaxSupplyKeeper

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, a.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, a.MaxSupplyKeeper)
	suite.queryClient = types.NewQueryClient(queryHelper)
}

func TestKeeperSuite(t *testing.T) {
	suite.Run(t, new(KeeperSuite))
}
