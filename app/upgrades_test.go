package app_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	memiavlstore "github.com/crypto-org-chain/cronos-store/store"
	memiavlrootmulti "github.com/crypto-org-chain/cronos-store/store/rootmulti"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protowire"

	"cosmossdk.io/collections"
	"cosmossdk.io/log/v2"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/store/v2/prefix"
	"github.com/cosmos/cosmos-sdk/store/v2/rootmulti"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
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

// Module consensus versions a v8 chain holds for the modules v9 migrates.
var v8ModuleVersions = map[string]uint64{
	authtypes.ModuleName:    5,
	stakingtypes.ModuleName: 5,
	minttypes.ModuleName:    2,
	feegrant.ModuleName:     2,
}

func stripProtoField(bz []byte, field protowire.Number) []byte {
	out := make([]byte, 0, len(bz))
	for len(bz) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(bz)
		valueLen := protowire.ConsumeFieldValue(num, typ, bz[tagLen:])
		if tagLen < 0 || valueLen < 0 {
			panic("malformed protobuf message")
		}
		if num != field {
			out = append(out, bz[:tagLen+valueLen]...)
		}
		bz = bz[tagLen+valueLen:]
	}
	return out
}

// Rewrites a stored message as a binary that predates the field wrote it.
func (suite *AppTestSuite) stripStoredProtoField(storeKey string, key []byte, field protowire.Number) {
	store := suite.ctx.KVStore(suite.app.GetKey(storeKey))
	bz := store.Get(key)
	suite.Require().NotEmpty(bz)
	stripped := stripProtoField(bz, field)
	suite.Require().Less(len(stripped), len(bz))
	store.Set(key, stripped)
}

func (suite *AppTestSuite) TestV9UpgradeHandler() {
	const (
		stakingKeyRotationFeeField = 7
		mintMaxSupplyField         = 7
	)
	var (
		feePool         = stakingtypes.KeyRotationFeePoolName
		feePoolAddr     = authtypes.NewModuleAddress(feePool)
		granter         = sdk.AccAddress("granter_____________")
		grantee         = sdk.AccAddress("grantee_____________")
		grantExpiration time.Time
	)
	// Takes the place of whatever account the fee pool address holds, keeping account numbers unique.
	baseAccountAtFeePool := func() *authtypes.BaseAccount {
		base := authtypes.NewBaseAccountWithAddress(feePoolAddr)
		if existing := suite.app.AccountKeeper.GetAccount(suite.ctx, feePoolAddr); existing != nil {
			suite.Require().NoError(base.SetAccountNumber(existing.GetAccountNumber()))
			return base
		}
		return suite.app.AccountKeeper.NewAccount(suite.ctx, base).(*authtypes.BaseAccount)
	}
	burnFromFeePool := func(ctx sdk.Context) error {
		fee := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1)))
		if err := suite.app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, fee); err != nil {
			return err
		}
		if err := suite.app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, feePool, fee); err != nil {
			return err
		}
		return suite.app.BankKeeper.BurnCoins(ctx, feePool, fee)
	}

	checkFeePoolIsBurner := func() {
		moduleAccount, ok := suite.app.AccountKeeper.GetAccount(suite.ctx, feePoolAddr).(sdk.ModuleAccountI)
		suite.Require().True(ok)
		suite.Require().Equal(feePool, moduleAccount.GetName())
		suite.Require().Equal([]string{authtypes.Burner}, moduleAccount.GetPermissions())
		suite.Require().NoError(burnFromFeePool(suite.ctx))
	}

	testCases := []struct {
		name     string
		malleate func()
		check    func()
	}{
		{
			name: "staking params gain the key rotation fee",
			malleate: func() {
				suite.stripStoredProtoField(stakingtypes.StoreKey, stakingtypes.ParamsKey, stakingKeyRotationFeeField)
				params, err := suite.app.StakingKeeper.GetParams(suite.ctx)
				suite.Require().NoError(err)
				suite.Require().Empty(params.KeyRotationFee.Denom)
				suite.Require().True(params.KeyRotationFee.Amount.IsNil())
			},
			check: func() {
				params, err := suite.app.StakingKeeper.GetParams(suite.ctx)
				suite.Require().NoError(err)
				suite.Require().Equal(sdk.NewCoin(params.BondDenom, stakingtypes.DefaultKeyRotationFeeAmount), params.KeyRotationFee)
				suite.Require().NoError(params.Validate())
			},
		},
		{
			name: "auth params gain the ML-DSA-65 cost and the account number sequence is removed",
			malleate: func() {
				params := suite.app.AccountKeeper.GetParams(suite.ctx)
				params.SigVerifyCostMlDsa65 = 0
				suite.Require().NoError(suite.app.AccountKeeper.Params.Set(suite.ctx, params))
				suite.Require().NoError(suite.app.AccountKeeper.AccountNumber.Set(suite.ctx, 42))
			},
			check: func() {
				suite.Require().Equal(authtypes.DefaultSigVerifyCostMlDsa65, suite.app.AccountKeeper.GetParams(suite.ctx).SigVerifyCostMlDsa65)
				has, err := collections.Item[uint64](suite.app.AccountKeeper.AccountNumber).Has(suite.ctx)
				suite.Require().NoError(err)
				suite.Require().False(has)
			},
		},
		{
			name: "mint params gain an unlimited max supply",
			malleate: func() {
				suite.stripStoredProtoField(minttypes.StoreKey, minttypes.ParamsKey.Bytes(), mintMaxSupplyField)
				params, err := suite.app.MintKeeper.Params.Get(suite.ctx)
				suite.Require().NoError(err)
				suite.Require().True(params.MaxSupply.IsNil())
			},
			check: func() {
				params, err := suite.app.MintKeeper.Params.Get(suite.ctx)
				suite.Require().NoError(err)
				suite.Require().False(params.MaxSupply.IsNil())
				suite.Require().True(params.MaxSupply.IsZero())
			},
		},
		{
			name: "feegrant expiry queue entries written with empty values can expire",
			malleate: func() {
				grantExpiration = suite.ctx.BlockTime().Add(time.Hour)
				allowance := &feegrant.BasicAllowance{Expiration: &grantExpiration}
				// app.FeeGrantKeeper has no bank keeper, which GrantAllowance needs to create a grantee account.
				suite.app.AccountKeeper.SetAccount(suite.ctx, suite.app.AccountKeeper.NewAccountWithAddress(suite.ctx, grantee))
				suite.Require().NoError(suite.app.FeeGrantKeeper.GrantAllowance(suite.ctx, granter, grantee, allowance))

				queue := prefix.NewStore(suite.ctx.KVStore(suite.app.GetKey(feegrant.StoreKey)), feegrant.FeeAllowanceQueueKeyPrefix)
				var keys [][]byte
				it := queue.Iterator(nil, nil)
				for ; it.Valid(); it.Next() {
					keys = append(keys, it.Key())
				}
				suite.Require().NoError(it.Close())
				suite.Require().Len(keys, 1)
				queue.Set(keys[0], []byte{})

				expired, _ := suite.ctx.WithBlockTime(grantExpiration.Add(time.Second)).CacheContext()
				suite.Require().Error(suite.app.FeeGrantKeeper.RemoveExpiredAllowances(expired, 10))
			},
			check: func() {
				expired := suite.ctx.WithBlockTime(grantExpiration.Add(time.Second))
				suite.Require().NoError(suite.app.FeeGrantKeeper.RemoveExpiredAllowances(expired, 10))
				allowance, err := suite.app.FeeGrantKeeper.GetAllowance(expired, granter, grantee)
				suite.Require().Error(err)
				suite.Require().Nil(allowance)
			},
		},
		{
			name: "plain account at the key rotation fee pool address becomes a burner module account",
			malleate: func() {
				suite.app.AccountKeeper.SetAccount(suite.ctx, baseAccountAtFeePool())
				cacheCtx, _ := suite.ctx.CacheContext()
				suite.Require().Panics(func() { _ = burnFromFeePool(cacheCtx) })
			},
			check: checkFeePoolIsBurner,
		},
		{
			// Anyone can create this account on v8, so it must not halt the upgrade.
			name: "vesting account at the key rotation fee pool address becomes a burner module account",
			malleate: func() {
				locked := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1)))
				vesting, err := vestingtypes.NewPermanentLockedAccount(baseAccountAtFeePool(), locked)
				suite.Require().NoError(err)
				suite.app.AccountKeeper.SetAccount(suite.ctx, vesting)
				cacheCtx, _ := suite.ctx.CacheContext()
				suite.Require().Panics(func() { _ = burnFromFeePool(cacheCtx) })
			},
			check: checkFeePoolIsBurner,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			versions := suite.app.ModuleManager.GetVersionMap()
			for name, version := range v8ModuleVersions {
				versions[name] = version
			}
			suite.Require().NoError(suite.app.UpgradeKeeper.SetModuleVersionMap(suite.ctx, versions))
			tc.malleate()

			plan := upgradetypes.Plan{Name: app.UpgradeV9PlanName, Height: suite.ctx.BlockHeight()}
			suite.Require().NoError(suite.app.UpgradeKeeper.ApplyUpgrade(suite.ctx, plan))

			name, _, err := suite.app.UpgradeKeeper.GetLastCompletedUpgrade(suite.ctx)
			suite.Require().NoError(err)
			suite.Require().Equal(app.UpgradeV9PlanName, name)

			migrated, err := suite.app.UpgradeKeeper.GetModuleVersionMap(suite.ctx)
			suite.Require().NoError(err)
			suite.Require().Equal(suite.app.ModuleManager.GetVersionMap(), migrated)

			// ML-DSA validator keys stay disabled until governance adds the key type.
			consensusParams, err := suite.app.ConsensusParamsKeeper.ParamsStore.Get(suite.ctx)
			suite.Require().NoError(err)
			suite.Require().Equal([]string{tmtypes.ABCIPubKeyTypeEd25519}, consensusParams.Validator.PubKeyTypes)

			tc.check()
		})
	}
}

func (suite *AppTestSuite) TestV9StoreUpgrades() {
	const (
		groupStore    = "group"
		upgradeHeight = 2
		mismatch      = "memiavl tree membership mismatch"
	)
	v9Plan := &upgradetypes.Plan{Name: app.UpgradeV9PlanName, Height: upgradeHeight}

	testCases := []struct {
		name        string
		memiavl     bool
		upgradeInfo *upgradetypes.Plan
		skipHeights []int
		expErr      string
	}{
		{name: "memiavl deletes the group store at the upgrade height", memiavl: true, upgradeInfo: v9Plan},
		{name: "memiavl refuses the group store without upgrade info", memiavl: true, expErr: mismatch},
		{
			name:        "memiavl refuses the group store for another plan",
			memiavl:     true,
			upgradeInfo: &upgradetypes.Plan{Name: "v8", Height: upgradeHeight},
			expErr:      mismatch,
		},
		{
			name:        "memiavl refuses the group store before the upgrade height",
			memiavl:     true,
			upgradeInfo: &upgradetypes.Plan{Name: app.UpgradeV9PlanName, Height: upgradeHeight + 1},
			expErr:      mismatch,
		},
		{
			name:        "memiavl refuses the group store when the upgrade height is skipped",
			memiavl:     true,
			upgradeInfo: v9Plan,
			skipHeights: []int{upgradeHeight},
			expErr:      mismatch,
		},
		{name: "iavl loads with the group store left unmounted", upgradeInfo: v9Plan},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			home := suite.T().TempDir()
			dataDir := filepath.Join(home, "data")
			suite.Require().NoError(os.MkdirAll(dataDir, 0o755))
			db := dbm.NewMemDB()

			// One committed version shaped like a v8 node's: every current store plus x/group's.
			var v8Store storetypes.CommitMultiStore = rootmulti.NewStore(db, log.NewNopLogger())
			if tc.memiavl {
				v8Store = memiavlrootmulti.NewStore(filepath.Join(dataDir, "memiavl.db"), log.NewNopLogger(), false, false, testutil.ChainID)
			}
			groupKey := storetypes.NewKVStoreKey(groupStore)
			v8Store.MountStoreWithDB(groupKey, storetypes.StoreTypeIAVL, nil)
			keys, _, _ := app.StoreKeys()
			for _, key := range keys {
				v8Store.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
			}
			suite.Require().NoError(v8Store.LoadLatestVersion())
			v8Store.GetKVStore(groupKey).Set([]byte("key"), []byte("value"))
			v8CommitID := v8Store.Commit()
			if closer, ok := v8Store.(io.Closer); ok {
				suite.Require().NoError(closer.Close())
			}

			if tc.upgradeInfo != nil {
				bz, err := json.Marshal(tc.upgradeInfo)
				suite.Require().NoError(err)
				suite.Require().NoError(os.WriteFile(filepath.Join(dataDir, upgradetypes.UpgradeInfoFilename), bz, 0o600))
			}

			newApp := func() *app.ChainApp {
				return app.New(log.NewNopLogger(), db, false, simtestutil.AppOptionsMap{
					flags.FlagHome:                home,
					flags.FlagChainID:             testutil.ChainID,
					memiavlstore.FlagMemIAVL:      tc.memiavl,
					server.FlagUnsafeSkipUpgrades: tc.skipHeights,
				}, baseapp.SetChainID(testutil.ChainID))
			}

			upgraded := newApp()
			err := upgraded.LoadLatestVersion()
			if tc.expErr != "" {
				suite.Require().ErrorContains(err, tc.expErr)
				return
			}
			suite.Require().NoError(err)
			// CometBFT compares this with the app hash it stored for the last v8 block.
			suite.Require().Equal(v8CommitID, upgraded.LastCommitID())

			// A restart after the upgrade block loads without the store upgrade.
			upgraded.CommitMultiStore().Commit()
			suite.Require().NoError(upgraded.Close())
			restarted := newApp()
			suite.Require().NoError(restarted.LoadLatestVersion())
			suite.Require().Equal(int64(upgradeHeight), restarted.LastBlockHeight())
			suite.Require().NoError(restarted.Close())
		})
	}
}

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

	asVesting := func(base *authtypes.BaseAccount) sdk.AccountI {
		locked := sdk.NewCoins(sdk.NewCoin("basecro", math.NewInt(1)))
		vesting, err := vestingtypes.NewPermanentLockedAccount(base, locked)
		suite.Require().NoError(err)
		return vesting
	}
	conversions := []struct {
		name string
		wrap func(*authtypes.BaseAccount) sdk.AccountI
	}{
		{name: "BaseAccount at module address is converted to ModuleAccount", wrap: func(base *authtypes.BaseAccount) sdk.AccountI { return base }},
		{name: "vesting account at module address is converted to ModuleAccount", wrap: asVesting},
	}
	for _, tc := range conversions {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			moduleName := tieredrewardstypes.RewardsPoolName
			addr := suite.app.AccountKeeper.GetModuleAddress(moduleName)

			// Reuse the module account's number: auth indexes accounts by a unique account number.
			existing := suite.app.AccountKeeper.GetAccount(suite.ctx, addr)
			suite.Require().NotNil(existing)
			const seq = uint64(7)
			base := authtypes.NewBaseAccountWithAddress(addr)
			suite.Require().NoError(base.SetAccountNumber(existing.GetAccountNumber()))
			suite.Require().NoError(base.SetSequence(seq))
			suite.app.AccountKeeper.SetAccount(suite.ctx, tc.wrap(base))

			suite.Require().NoError(app.EnsureModuleAccountIfExists(suite.ctx, suite.app.AccountKeeper, moduleName))

			modAcc, ok := suite.app.AccountKeeper.GetAccount(suite.ctx, addr).(sdk.ModuleAccountI)
			suite.Require().True(ok, "account at module address must be a ModuleAccount after conversion")
			suite.Require().Equal(moduleName, modAcc.GetName())
			suite.Require().Equal(existing.GetAccountNumber(), modAcc.GetAccountNumber())
			suite.Require().Equal(seq, modAcc.GetSequence())
		})
	}
}
