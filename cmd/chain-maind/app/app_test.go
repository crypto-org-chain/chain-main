package app_test

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	cmtcli "github.com/cometbft/cometbft/libs/cli"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/app/params"
	cmd "github.com/crypto-org-chain/chain-main/v8/cmd/chain-maind/app"
	"github.com/crypto-org-chain/chain-main/v8/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/client/flags"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// NewRootCmd seals the SDK config and registers denoms, so it can run only once per process.
var newRootCmd = sync.OnceValues(func() (*cobra.Command, params.EncodingConfig) {
	rootCmd, encodingConfig := cmd.NewRootCmd()
	rootCmd.PersistentFlags().String(flags.FlagLogLevel, "info", "")
	rootCmd.PersistentFlags().String(flags.FlagLogFormat, "plain", "")
	rootCmd.PersistentFlags().Bool(flags.FlagLogNoColor, false, "")
	rootCmd.PersistentFlags().String(flags.FlagVerboseLogLevel, "debug", "")
	cmtcli.PrepareBaseCmd(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome)
	return rootCmd, encodingConfig
})

type GenesisCmdTestSuite struct {
	suite.Suite

	rootCmd        *cobra.Command
	encodingConfig params.EncodingConfig
}

func TestGenesisCmdTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisCmdTestSuite))
}

func (s *GenesisCmdTestSuite) SetupSuite() {
	s.rootCmd, s.encodingConfig = newRootCmd()
}

func (s *GenesisCmdTestSuite) execute(args ...string) error {
	s.rootCmd.SetArgs(args)
	return s.rootCmd.ExecuteContext(svrcmd.CreateExecuteContext(s.T().Context()))
}

func testnetArgs(home string, validators int, extra ...string) []string {
	args := []string{"testnet", "--v", strconv.Itoa(validators), "-o", home, "--keyring-backend", "test", "--home", home}
	return append(args, extra...)
}

func (s *GenesisCmdTestSuite) TestGenesisIsValid() {
	const validators = 2
	testnet := func(home string) []string { return testnetArgs(home, validators) }
	testnetGenesis := filepath.Join("node0", ".chain-maind", "config", "genesis.json")

	testCases := []struct {
		name        string
		args        func(home string) []string
		genesisFile string
		expAccounts int
	}{
		{
			name: "init",
			args: func(home string) []string {
				return []string{"init", "node", "--chain-id", "test", "--home", home}
			},
			genesisFile: filepath.Join("config", "genesis.json"),
		},
		{name: "testnet first run", args: testnet, genesisFile: testnetGenesis, expAccounts: validators},
		{name: "testnet rerun in the same process", args: testnet, genesisFile: testnetGenesis, expAccounts: validators},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			home := s.T().TempDir()
			s.Require().NoError(s.execute(tc.args(home)...))
			genesisFile := filepath.Join(home, tc.genesisFile)

			s.Require().NoError(s.execute("genesis", "validate", genesisFile, "--home", home))

			appGenesis, err := genutiltypes.AppGenesisFromFile(genesisFile)
			s.Require().NoError(err)
			var appState map[string]json.RawMessage
			s.Require().NoError(json.Unmarshal(appGenesis.AppState, &appState))
			cdc := s.encodingConfig.Marshaler
			var auth authtypes.GenesisState
			s.Require().NoError(cdc.UnmarshalJSON(appState[authtypes.ModuleName], &auth))
			var bank banktypes.GenesisState
			s.Require().NoError(cdc.UnmarshalJSON(appState[banktypes.ModuleName], &bank))
			var staking stakingtypes.GenesisState
			s.Require().NoError(cdc.UnmarshalJSON(appState[stakingtypes.ModuleName], &staking))
			var gov govv1.GenesisState
			s.Require().NoError(cdc.UnmarshalJSON(appState[govtypes.ModuleName], &gov))

			s.Require().Len(auth.Accounts, tc.expAccounts)
			s.Require().Len(bank.Balances, tc.expAccounts)
			s.Require().Equal(config.BaseCoinUnit, staking.Params.BondDenom)
			s.Require().Equal(
				sdk.NewCoin(config.BaseCoinUnit, stakingtypes.DefaultKeyRotationFeeAmount),
				staking.Params.KeyRotationFee,
			)
			s.Require().Equal(
				sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, govv1.DefaultMinDepositTokens)),
				sdk.NewCoins(gov.Params.MinDeposit...),
			)
			s.Require().Equal(
				sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, govv1.DefaultMinExpeditedDepositTokens)),
				sdk.NewCoins(gov.Params.ExpeditedMinDeposit...),
			)
		})
	}
}

func (s *GenesisCmdTestSuite) TestTestnetRejectsInvalidUnbondingTime() {
	const flagUnbondingTime = "unbonding-time"
	testnetCmd, _, err := s.rootCmd.Find([]string{"testnet"})
	s.Require().NoError(err)
	// Cobra keeps a flag's value for later runs of the same command.
	defer func() {
		flag := testnetCmd.Flags().Lookup(flagUnbondingTime)
		s.Require().NoError(flag.Value.Set(flag.DefValue))
	}()

	home := s.T().TempDir()
	err = s.execute(testnetArgs(home, 1, "--"+flagUnbondingTime, "soon")...)
	s.Require().ErrorContains(err, flagUnbondingTime)
}
