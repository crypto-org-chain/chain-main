package app_test

import (
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardskeeper "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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

func (suite *AppTestSuite) TestV730UpgradeHandlerExitsVestedAccountPositions() {
	suite.SetupTest()

	// 1. Set up tier 1 with a permissive min lock for the test.
	suite.Require().NoError(suite.app.TieredRewardsKeeper.SetTier(suite.ctx, tieredrewardstypes.Tier{
		Id:            1,
		ExitDuration:  365 * 24 * time.Hour,
		BonusApy:      math.LegacyMustNewDecFromStr("0.02"),
		MinLockAmount: math.NewInt(1_000_000),
	}))

	// 2. Find a bonded validator.
	vals, err := suite.app.StakingKeeper.GetBondedValidatorsByPower(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(vals)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	suite.Require().NoError(err)
	bondDenom, err := suite.app.StakingKeeper.BondDenom(suite.ctx)
	suite.Require().NoError(err)

	amount := math.NewInt(1_000_000)
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, amount))
	msgServer := tieredrewardskeeper.NewMsgServerImpl(suite.app.TieredRewardsKeeper)

	// commitForOwner mints funds, delegates, and commits via the message
	// handler. Owner must be a non-vesting account at this point because
	// the v7.3.0 guard rejects vesting accounts.
	commitForOwner := func(owner sdk.AccAddress) uint64 {
		suite.Require().NoError(banktestutil.FundAccount(suite.ctx, suite.app.BankKeeper, owner, coins))
		_, err := suite.app.StakingKeeper.Delegate(suite.ctx, owner, amount, stakingtypes.Unbonded, val, true)
		suite.Require().NoError(err)
		resp, err := msgServer.CommitDelegationToTier(suite.ctx, &tieredrewardstypes.MsgCommitDelegationToTier{
			DelegatorAddress: owner.String(),
			ValidatorAddress: valAddr.String(),
			Id:               1,
			Amount:           amount,
		})
		suite.Require().NoError(err)
		return resp.PositionId
	}

	// 3. Set up two positions: one owned by a regular account, one owned
	//    by what will become a permanent-locked vesting account.
	regularOwner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	regularBase, ok := suite.app.AccountKeeper.NewAccountWithAddress(suite.ctx, regularOwner).(*authtypes.BaseAccount)
	suite.Require().True(ok)
	suite.app.AccountKeeper.SetAccount(suite.ctx, regularBase)
	regularPosID := commitForOwner(regularOwner)

	vestingOwner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	vestingBase, ok := suite.app.AccountKeeper.NewAccountWithAddress(suite.ctx, vestingOwner).(*authtypes.BaseAccount)
	suite.Require().True(ok)
	suite.app.AccountKeeper.SetAccount(suite.ctx, vestingBase)
	vestingPosID := commitForOwner(vestingOwner)

	// Now wrap vestingOwner as a PermanentLockedAccount in place. Account
	// number / sequence carry over to satisfy the auth uniqueness index.
	existing := suite.app.AccountKeeper.GetAccount(suite.ctx, vestingOwner)
	vestingBase = authtypes.NewBaseAccountWithAddress(vestingOwner)
	suite.Require().NoError(vestingBase.SetAccountNumber(existing.GetAccountNumber()))
	suite.Require().NoError(vestingBase.SetSequence(existing.GetSequence()))
	vestingAcc, err := vestingtypes.NewPermanentLockedAccount(vestingBase, coins)
	suite.Require().NoError(err)
	suite.app.AccountKeeper.SetAccount(suite.ctx, vestingAcc)

	// 4. Run the exit function.
	suite.Require().NoError(app.ExitVestedAccountsPositions(suite.ctx, suite.app))

	// 5. Vesting position deleted, owner has staking delegation back.
	_, err = suite.app.TieredRewardsKeeper.Positions.Get(suite.ctx, vestingPosID)
	suite.Require().Error(err, "vesting-owned position must be deleted")

	vestingDeleg, err := suite.app.StakingKeeper.GetDelegation(suite.ctx, vestingOwner, valAddr)
	suite.Require().NoError(err, "vesting owner must have staking delegation back")
	suite.Require().True(vestingDeleg.Shares.IsPositive())

	// 6. Regular-owner position untouched.
	regularPos, err := suite.app.TieredRewardsKeeper.Positions.Get(suite.ctx, regularPosID)
	suite.Require().NoError(err, "regular position must survive")
	suite.Require().Equal(regularOwner.String(), regularPos.Owner)
}
