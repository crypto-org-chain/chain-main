package app_test

import (
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
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

// TestEnsureModuleAccountIfExists tests that the conversion of orphan BaseAccounts (left at module
// addresses by external pre-funding) into proper ModuleAccounts works as expected.
func (suite *AppTestSuite) TestEnsureModuleAccountIfExists() {
	suite.Run("module not registered in maccPerms returns error", func() {
		suite.SetupTest()
		err := app.EnsureModuleAccountIfExists(suite.ctx, suite.app.AccountKeeper, "definitely_not_a_module")
		suite.Require().ErrorContains(err, "not registered in maccPerms")
	})

	suite.Run("no account at module address is a no-op", func() {
		suite.SetupTest()
		moduleName := tieredrewardstypes.RewardsPoolName
		addr := suite.app.AccountKeeper.GetModuleAddress(moduleName)
		suite.Require().NotNil(addr)
		// Wipe whatever the keeper auto-created at genesis.
		if existing := suite.app.AccountKeeper.GetAccount(suite.ctx, addr); existing != nil {
			suite.app.AccountKeeper.RemoveAccount(suite.ctx, existing)
		}
		suite.Require().Nil(suite.app.AccountKeeper.GetAccount(suite.ctx, addr))

		suite.Require().NoError(app.EnsureModuleAccountIfExists(suite.ctx, suite.app.AccountKeeper, moduleName))
		// Helper must not create the account itself — that's the responsibility
		// of the module's InitGenesis path.
		suite.Require().Nil(suite.app.AccountKeeper.GetAccount(suite.ctx, addr))
	})

	suite.Run("already a ModuleAccount is a no-op", func() {
		suite.SetupTest()
		moduleName := authtypes.FeeCollectorName
		before := suite.app.AccountKeeper.GetModuleAccount(suite.ctx, moduleName)
		suite.Require().NotNil(before)

		suite.Require().NoError(app.EnsureModuleAccountIfExists(suite.ctx, suite.app.AccountKeeper, moduleName))

		after := suite.app.AccountKeeper.GetModuleAccount(suite.ctx, moduleName)
		suite.Require().Equal(before.GetAddress(), after.GetAddress())
		suite.Require().Equal(before.GetAccountNumber(), after.GetAccountNumber())
		suite.Require().Equal(before.GetSequence(), after.GetSequence())
	})

	suite.Run("BaseAccount at module address is converted to ModuleAccount", func() {
		suite.SetupTest()
		moduleName := tieredrewardstypes.RewardsPoolName
		addr := suite.app.AccountKeeper.GetModuleAddress(moduleName)

		// Replace the auto-created ModuleAccount with a BaseAccount to simulate
		// an orphan address pre-funded before the module was registered.
		const accNum, seq = uint64(999), uint64(7)
		base := authtypes.NewBaseAccountWithAddress(addr)
		suite.Require().NoError(base.SetAccountNumber(accNum))
		suite.Require().NoError(base.SetSequence(seq))
		suite.app.AccountKeeper.SetAccount(suite.ctx, base)

		suite.Require().NoError(app.EnsureModuleAccountIfExists(suite.ctx, suite.app.AccountKeeper, moduleName))

		converted := suite.app.AccountKeeper.GetAccount(suite.ctx, addr)
		modAcc, ok := converted.(sdk.ModuleAccountI)
		suite.Require().True(ok, "account at module address must be a ModuleAccount after conversion")
		suite.Require().Equal(moduleName, modAcc.GetName())
		// AccountNumber and Sequence must be preserved so any earlier on-chain
		// references to the address remain consistent.
		suite.Require().Equal(accNum, modAcc.GetAccountNumber())
		suite.Require().Equal(seq, modAcc.GetSequence())
	})

	suite.Run("non-BaseAccount type is rejected with an error", func() {
		suite.SetupTest()
		moduleName := tieredrewardstypes.RewardsPoolName
		addr := suite.app.AccountKeeper.GetModuleAddress(moduleName)

		// Set a vesting account at the module address — this is the kind of
		// shape we shouldn't blindly convert (vesting metadata would be lost).
		// Take over the existing module account's number/sequence to avoid
		// the account-number uniqueness constraint at write time.
		existing := suite.app.AccountKeeper.GetAccount(suite.ctx, addr)
		suite.Require().NotNil(existing)
		base := authtypes.NewBaseAccountWithAddress(addr)
		suite.Require().NoError(base.SetAccountNumber(existing.GetAccountNumber()))
		suite.Require().NoError(base.SetSequence(existing.GetSequence()))
		coins := sdk.NewCoins(sdk.NewCoin("basecro", math.NewInt(1)))
		vest, err := vestingtypes.NewPermanentLockedAccount(base, coins)
		suite.Require().NoError(err)
		suite.app.AccountKeeper.SetAccount(suite.ctx, vest)

		err = app.EnsureModuleAccountIfExists(suite.ctx, suite.app.AccountKeeper, moduleName)
		suite.Require().ErrorContains(err, "cannot convert to module account")
	})
}
