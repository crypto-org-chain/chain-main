package app_test

import (
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/chain-main/v4/testutil"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

type AppTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	app       *app.ChainApp
	govParams govv1.Params
}

func TestAppTestSuite(t *testing.T) {
	suite.Run(t, new(AppTestSuite))
}

func (suite *AppTestSuite) SetupTest() {
	checkTx := false
	suite.app = testutil.Setup(checkTx, nil)
	suite.ctx = suite.app.NewContext(checkTx).WithBlockHeader(tmproto.Header{Height: 1, ChainID: testutil.ChainID, Time: time.Now().UTC()})
	var err error
	suite.govParams, err = suite.app.GovKeeper.Params.Get(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(govv1.DefaultParams(), suite.govParams)
}

func (suite *AppTestSuite) TestUpdateExpeditedParams() {
	const baseDenom = "basecro"

	testCases := []struct {
		name     string
		malleate func()
		exp      func(params govv1.Params)
	}{
		{
			name: "update ExpeditedMinDeposit with baseDenom",
			malleate: func() {
				suite.govParams.MinDeposit = sdk.NewCoins(sdk.NewCoin(baseDenom, math.NewInt(2000000000000)))
			},
			exp: func(params govv1.Params) {
				expected := sdk.NewCoins(sdk.NewCoin(suite.govParams.MinDeposit[0].Denom, suite.govParams.MinDeposit[0].Amount.MulRaw(govv1.DefaultMinExpeditedDepositTokensRatio)))
				suite.Require().Equal(expected[0], params.ExpeditedMinDeposit[0])
			},
		},
		{
			name: "update ExpeditedThreshold when DefaultExpeditedThreshold < Threshold",
			malleate: func() {
				suite.govParams.Threshold = "0.99"
			},
			exp: func(params govv1.Params) {
				suite.Require().Equal(math.LegacyOneDec().String(), params.ExpeditedThreshold)
			},
		},
		{
			name: "update ExpeditedThreshold when DefaultExpeditedThreshold = Threshold",
			malleate: func() {
				suite.govParams.Threshold = govv1.DefaultExpeditedThreshold.String()
			},
			exp: func(params govv1.Params) {
				expected := app.DefaultThresholdRatio().Mul(math.LegacyMustNewDecFromStr(suite.govParams.Threshold))
				suite.Require().Equal(expected.String(), params.ExpeditedThreshold)
			},
		},
		{
			name: "no update ExpeditedThreshold when DefaultExpeditedThreshold > Threshold",
			malleate: func() {
				suite.govParams.Threshold = govv1.DefaultExpeditedThreshold.Quo(math.LegacyMustNewDecFromStr("1.1")).String()
			},
			exp: func(params govv1.Params) {
				suite.Require().Equal(suite.govParams.ExpeditedThreshold, params.ExpeditedThreshold)
			},
		},
		{
			name: "update ExpeditedVotingPeriod when DefaultExpeditedPeriod > VotingPeriod",
			malleate: func() {
				period := govv1.DefaultExpeditedPeriod
				votingPeriod := period - 1*time.Second
				suite.govParams.VotingPeriod = &votingPeriod
			},
			exp: func(params govv1.Params) {
				votingPeriod := app.DurationToDec(*suite.govParams.VotingPeriod)
				expected := app.DecToDuration(app.DefaultPeriodRatio().Mul(votingPeriod))
				suite.Require().Equal(expected, *params.ExpeditedVotingPeriod)
			},
		},
		{
			name: "update ExpeditedVotingPeriod when DefaultExpeditedPeriod = VotingPeriod",
			malleate: func() {
				period := govv1.DefaultExpeditedPeriod
				suite.govParams.VotingPeriod = &period
			},
			exp: func(params govv1.Params) {
				votingPeriod := app.DurationToDec(*suite.govParams.VotingPeriod)
				expected := app.DecToDuration(app.DefaultPeriodRatio().Mul(votingPeriod))
				suite.Require().Equal(expected, *params.ExpeditedVotingPeriod)
			},
		},
		{
			name: "no update ExpeditedVotingPeriod when DefaultExpeditedPeriod < VotingPeriod",
			malleate: func() {
				period := govv1.DefaultExpeditedPeriod + 1
				suite.govParams.VotingPeriod = &period
			},
			exp: func(params govv1.Params) {
				suite.Require().Equal(*suite.govParams.ExpeditedVotingPeriod, *params.ExpeditedVotingPeriod)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.malleate()
			suite.Require().NoError(suite.app.GovKeeper.Params.Set(suite.ctx, suite.govParams))
			suite.Require().NoError(app.UpdateExpeditedParams(suite.ctx, suite.app.GovKeeper))
			params, err := suite.app.GovKeeper.Params.Get(suite.ctx)
			suite.Require().NoError(err)
			tc.exp(params)
		})
	}
}
