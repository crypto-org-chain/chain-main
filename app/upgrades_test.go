package app_test

import (
	stdmath "math"
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/config"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
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

// reserveVestingPeriods mirrors the schedule the v5.0.0 upgrade handler used to
// create the 70B CRO reserve account on mainnet: 60 monthly periods, with the
// last period carrying the integer-division remainder.
const (
	reserveNumPeriods     = 60
	reservePeriodDuration = int64(2628000) // 1 month, as used on mainnet
)

// setupReserveVesting stores a PeriodicVestingAccount shaped like the mainnet
// 70B CRO reserve, positioned so that elapsedPeriods periods have fully vested
// as of the suite's current block time.
func (suite *AppTestSuite) setupReserveVesting(elapsedPeriods int64) (*vestingtypes.PeriodicVestingAccount, sdk.AccAddress) {
	return suite.setupReserveVestingAt(sdk.AccAddress("reserve_vesting_acc_"), nil, elapsedPeriods)
}

// setupReserveVestingAt is setupReserveVesting at a caller-chosen address, with
// an optional pubkey (which must match the address).
func (suite *AppTestSuite) setupReserveVestingAt(
	addr sdk.AccAddress, pk cryptotypes.PubKey, elapsedPeriods int64,
) (*vestingtypes.PeriodicVestingAccount, sdk.AccAddress) {
	total := math.NewInt(7000000000000000000)
	perPeriod := total.QuoRaw(reserveNumPeriods)
	lastPeriod := total.Sub(perPeriod.MulRaw(reserveNumPeriods - 1))

	periods := make(vestingtypes.Periods, reserveNumPeriods)
	for i := range reserveNumPeriods - 1 {
		periods[i] = vestingtypes.Period{
			Length: reservePeriodDuration,
			Amount: sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, perPeriod)),
		}
	}
	periods[reserveNumPeriods-1] = vestingtypes.Period{
		Length: reservePeriodDuration,
		Amount: sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, lastPeriod)),
	}

	// NewAccountWithAddress assigns the next free account number, avoiding the
	// account-number uniqueness constraint on write.
	base := suite.app.AccountKeeper.NewAccountWithAddress(suite.ctx, addr).(*authtypes.BaseAccount)
	suite.Require().NoError(base.SetSequence(3))
	if pk != nil {
		suite.Require().NoError(base.SetPubKey(pk))
	}

	startTime := suite.ctx.BlockTime().Add(-time.Duration(elapsedPeriods*reservePeriodDuration) * time.Second)
	acc, err := vestingtypes.NewPeriodicVestingAccount(
		base,
		sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, total)),
		startTime.Unix(),
		periods,
	)
	suite.Require().NoError(err)
	suite.app.AccountKeeper.SetAccount(suite.ctx, acc)
	return acc, addr
}

// reserveAccount reads the account back and asserts it is still periodic vesting.
func (suite *AppTestSuite) reserveAccount(addr sdk.AccAddress) *vestingtypes.PeriodicVestingAccount {
	acc := suite.app.AccountKeeper.GetAccount(suite.ctx, addr)
	suite.Require().NotNil(acc)
	pva, ok := acc.(*vestingtypes.PeriodicVestingAccount)
	suite.Require().True(ok, "expected PeriodicVestingAccount, got %T", acc)
	return pva
}

func (suite *AppTestSuite) TestPauseVestingAccount() {
	suite.Run("pause mid-schedule keeps vested amount and shifts end time", func() {
		suite.SetupTest()
		acc, addr := suite.setupReserveVesting(18)
		oldEndTime := acc.EndTime
		vestedBefore := acc.GetVestedCoins(suite.ctx.BlockTime())
		suite.Require().False(vestedBefore.IsZero(), "fixture should have vested coins to guard")

		suite.Require().NoError(app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		))

		paused := suite.reserveAccount(addr)
		suite.Require().Equal(oldEndTime+app.PauseVestingDuration, paused.EndTime)
		// Already-vested coins must not move.
		suite.Require().True(vestedBefore.Equal(paused.GetVestedCoins(suite.ctx.BlockTime())),
			"vested changed: %s -> %s", vestedBefore, paused.GetVestedCoins(suite.ctx.BlockTime()))
		// And nothing further may unlock during the pause.
		later := suite.ctx.BlockTime().Add(10 * 365 * 24 * time.Hour)
		suite.Require().True(vestedBefore.Equal(paused.GetVestedCoins(later)),
			"coins unlocked during pause: %s", paused.GetVestedCoins(later))
	})

	suite.Run("fully vested account is rejected", func() {
		suite.SetupTest()
		// Every period has elapsed, so there is nothing left to pause.
		_, addr := suite.setupReserveVesting(reserveNumPeriods + 1)
		before := suite.reserveAccount(addr)

		err := app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		)

		suite.Require().ErrorContains(err, "fully vested")
		// State must be untouched on error.
		suite.Require().Equal(before.EndTime, suite.reserveAccount(addr).EndTime)
	})

	suite.Run("pausing an already paused account is a no-op", func() {
		suite.SetupTest()
		_, addr := suite.setupReserveVesting(18)

		suite.Require().NoError(app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		))
		afterFirst := suite.reserveAccount(addr)

		// A re-run (e.g. the handler executing twice) must not stack a second
		// 50-year shift on top of the first.
		suite.Require().NoError(app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		))

		afterSecond := suite.reserveAccount(addr)
		suite.Require().Equal(afterFirst.EndTime, afterSecond.EndTime)
		suite.Require().Equal(afterFirst.VestingPeriods, afterSecond.VestingPeriods)
	})

	suite.Run("a delta that overflows the schedule is rejected", func() {
		suite.SetupTest()
		_, addr := suite.setupReserveVesting(18)
		before := suite.reserveAccount(addr)

		err := app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, stdmath.MaxInt64,
		)

		suite.Require().Error(err)
		// The corrupt account must never reach the store.
		after := suite.reserveAccount(addr)
		suite.Require().Equal(before.EndTime, after.EndTime)
		suite.Require().Equal(before.VestingPeriods, after.VestingPeriods)
		suite.Require().NoError(after.Validate())
	})

	suite.Run("a shift that would unlock coins early is rejected", func() {
		suite.SetupTest()
		_, addr := suite.setupReserveVesting(18)
		before := suite.reserveAccount(addr)
		vestedBefore := before.GetVestedCoins(suite.ctx.BlockTime())

		// Collapsing the in-progress period to zero length keeps the account
		// structurally valid (Validate permits a zero-length period), but
		// completes that period immediately and so vests it early. Pausing must
		// never change what is already unlocked, in either direction.
		err := app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, -reservePeriodDuration,
		)

		suite.Require().ErrorContains(err, "vested")
		after := suite.reserveAccount(addr)
		suite.Require().True(vestedBefore.Equal(after.GetVestedCoins(suite.ctx.BlockTime())))
		suite.Require().Equal(before.EndTime, after.EndTime)
	})

	suite.Run("identity and delegation counters survive the pause", func() {
		suite.SetupTest()
		// Address must derive from the pubkey, or BaseAccount.Validate rejects it.
		pk := secp256k1.GenPrivKey().PubKey()
		acc, addr := suite.setupReserveVestingAt(sdk.AccAddress(pk.Address()), pk, 18)

		// Give the account the delegation state a real staked reserve would
		// carry, so this would catch a rewrite that rebuilt it from parts.
		acc.DelegatedVesting = sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, math.NewInt(500)))
		acc.DelegatedFree = sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, math.NewInt(70)))
		suite.app.AccountKeeper.SetAccount(suite.ctx, acc)

		suite.Require().NoError(app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		))

		paused := suite.reserveAccount(addr)
		suite.Require().Equal(addr, paused.GetAddress())
		suite.Require().Equal(acc.GetAccountNumber(), paused.GetAccountNumber())
		suite.Require().Equal(acc.GetSequence(), paused.GetSequence())
		suite.Require().True(pk.Equals(paused.GetPubKey()))
		suite.Require().Equal(acc.OriginalVesting, paused.OriginalVesting)
		suite.Require().Equal(acc.DelegatedVesting, paused.DelegatedVesting)
		suite.Require().Equal(acc.DelegatedFree, paused.DelegatedFree)
		suite.Require().Equal(acc.StartTime, paused.StartTime)
		suite.Require().Len(paused.VestingPeriods, reserveNumPeriods)
	})

	suite.Run("periods that already vested are left untouched", func() {
		suite.SetupTest()
		acc, addr := suite.setupReserveVesting(18)
		originalPeriods := make(vestingtypes.Periods, len(acc.VestingPeriods))
		copy(originalPeriods, acc.VestingPeriods)

		suite.Require().NoError(app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		))

		paused := suite.reserveAccount(addr)
		// Exactly one period absorbs the shift: index 18, the one in progress.
		for i, p := range paused.VestingPeriods {
			if i == 18 {
				suite.Require().Equal(originalPeriods[i].Length+app.PauseVestingDuration, p.Length,
					"period %d should absorb the pause", i)
			} else {
				suite.Require().Equal(originalPeriods[i].Length, p.Length,
					"period %d must not move", i)
			}
			suite.Require().Equal(originalPeriods[i].Amount, p.Amount, "period %d amount must not change", i)
		}
	})

	suite.Run("account that has not started vesting shifts its first period", func() {
		suite.SetupTest()
		// StartTime one period in the future: nothing has vested yet.
		acc, addr := suite.setupReserveVesting(-1)
		suite.Require().True(acc.GetVestedCoins(suite.ctx.BlockTime()).IsZero())

		suite.Require().NoError(app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		))

		paused := suite.reserveAccount(addr)
		suite.Require().Equal(
			acc.VestingPeriods[0].Length+app.PauseVestingDuration,
			paused.VestingPeriods[0].Length,
		)
		suite.Require().True(paused.GetVestedCoins(suite.ctx.BlockTime()).IsZero())
	})

	suite.Run("non-periodic vesting account types are rejected", func() {
		suite.SetupTest()
		addr := sdk.AccAddress("permanently_locked_ac")
		base := suite.app.AccountKeeper.NewAccountWithAddress(suite.ctx, addr).(*authtypes.BaseAccount)
		locked, err := vestingtypes.NewPermanentLockedAccount(
			base, sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, math.NewInt(1))),
		)
		suite.Require().NoError(err)
		suite.app.AccountKeeper.SetAccount(suite.ctx, locked)

		err = app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper, addr, app.PauseVestingDuration,
		)

		suite.Require().ErrorContains(err, "expected a periodic vesting account")
	})

	suite.Run("missing account is rejected", func() {
		suite.SetupTest()
		err := app.PauseVestingAccount(
			suite.ctx, suite.app.AccountKeeper,
			sdk.AccAddress("no_such_account_here_"), app.PauseVestingDuration,
		)
		suite.Require().ErrorContains(err, "expected a periodic vesting account")
	})
}

// TestPauseVestingSpec guards the chain-ID lookup table. A wrong or missing
// mainnet key would make the upgrade a silent no-op on mainnet, which is the
// worst possible failure mode for this handler, so it is asserted explicitly
// rather than left to review.
func (suite *AppTestSuite) TestPauseVestingSpec() {
	const mainnetChainID = "crypto-org-chain-mainnet-1"

	suite.Run("mainnet reserve address matches the v5.0.0 handler", func() {
		addr, ok := app.PauseVestingSpec[mainnetChainID]
		suite.Require().True(ok, "mainnet chain ID missing from PauseVestingSpec")
		suite.Require().Equal("cro198pra975lcj526974r80fflr6retphnl3l7f4h", addr)
	})

	suite.Run("every spec address is a well-formed account address", func() {
		for chainID, addr := range app.PauseVestingSpec {
			// bech32 decoding directly, so this does not depend on whichever
			// global SDK prefix the test binary happens to have configured.
			hrp, bz, err := bech32.DecodeAndConvert(addr)
			suite.Require().NoError(err, "chain %s has an undecodable address %q", chainID, addr)
			suite.Require().Len(bz, 20, "chain %s address is not 20 bytes", chainID)
			if chainID == mainnetChainID {
				suite.Require().Equal("cro", hrp)
			}
		}
	})
}

func (suite *AppTestSuite) TestPauseReserveVesting() {
	suite.Run("chain without a spec entry is skipped", func() {
		suite.SetupTest()
		// testutil.ChainID ("chainmain-1") is deliberately not in the spec.
		_, ok := app.PauseVestingSpec[testutil.ChainID]
		suite.Require().False(ok, "test precondition: chain must have no spec entry")
		_, addr := suite.setupReserveVesting(18)
		before := suite.reserveAccount(addr)

		// A devnet that never ran the v5.0.0 handler must not fail the upgrade.
		suite.Require().NoError(app.PauseReserveVesting(suite.ctx, suite.app.AccountKeeper))

		suite.Require().Equal(before.EndTime, suite.reserveAccount(addr).EndTime)
	})
}

// TestV9UpgradeHandlerRegistered guards the wiring: a handler that is written
// but never registered would leave the chain halted at the upgrade height.
func (suite *AppTestSuite) TestV9UpgradeHandlerRegistered() {
	suite.SetupTest()
	suite.Require().True(
		suite.app.UpgradeKeeper.HasHandler(app.UpgradeV9PlanName),
		"no handler registered for plan %q", app.UpgradeV9PlanName,
	)
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
