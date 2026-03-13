package keeper_test

import (
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"time"
)

var isCheckTx = false

type KeeperSuite struct {
	suite.Suite

	ctx         sdk.Context
	keeper      keeper.Keeper
	app         *app.ChainApp
	queryClient types.QueryClient
}

func (suite *KeeperSuite) SetupTest() {
	a := testutil.Setup(isCheckTx, nil)
	suite.app = a
	suite.ctx = a.BaseApp.NewContext(isCheckTx).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	suite.keeper = a.TieredRewardsKeeper

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, a.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, keeper.NewQueryServerImpl(a.TieredRewardsKeeper))
	suite.queryClient = types.NewQueryClient(queryHelper)
}

func TestKeeperSuite(t *testing.T) {
	suite.Run(t, new(KeeperSuite))
}

func (s *KeeperSuite) setupTierAndDelegator() (sdk.AccAddress, sdk.ValAddress, string) {
	s.T().Helper()

	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().NotEmpty(dels)
	delAddrBytes, err := s.app.AccountKeeper.AddressCodec().StringToBytes(dels[0].DelegatorAddress)
	s.Require().NoError(err)
	delAddr := sdk.AccAddress(delAddrBytes)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(1)
	s.ctx = s.ctx.WithBlockTime(time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC))

	return delAddr, valAddr, bondDenom
}
