package app_test

import (
	"encoding/json"
	"path/filepath"
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
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
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

func (s *GenesisCmdTestSuite) TestInitGenesisIsValid() {
	home := s.T().TempDir()
	genesisFile := filepath.Join(home, "config", "genesis.json")

	s.Require().NoError(s.execute("init", "node", "--chain-id", "test", "--home", home))
	s.Require().NoError(s.execute("genesis", "validate", genesisFile, "--home", home))

	appGenesis, err := genutiltypes.AppGenesisFromFile(genesisFile)
	s.Require().NoError(err)
	var appState map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(appGenesis.AppState, &appState))
	var staking stakingtypes.GenesisState
	s.Require().NoError(s.encodingConfig.Marshaler.UnmarshalJSON(appState[stakingtypes.ModuleName], &staking))

	s.Require().Equal(config.BaseCoinUnit, staking.Params.BondDenom)
	s.Require().Equal(
		sdk.NewCoin(config.BaseCoinUnit, stakingtypes.DefaultKeyRotationFeeAmount),
		staking.Params.KeyRotationFee,
	)
}
