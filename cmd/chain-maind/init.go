package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crypto-com/chain-main/app"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/cli"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/staking"
)

const (
	flagOverwrite = "overwrite"
)

func GetGenesisStakingStateFromAppState(cdc *codec.Codec, appState map[string]json.RawMessage) staking.GenesisState {
	var genesisState staking.GenesisState
	if appState[staking.ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[staking.ModuleName], &genesisState)
	}

	return genesisState
}

func GetGenesisGovStateFromAppState(cdc *codec.Codec, appState map[string]json.RawMessage) gov.GenesisState {
	var genesisState gov.GenesisState
	if appState[gov.ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[gov.ModuleName], &genesisState)
	}

	return genesisState
}

type printInfo struct {
	Moniker    string          `json:"moniker" yaml:"moniker"`
	ChainID    string          `json:"chain_id" yaml:"chain_id"`
	NodeID     string          `json:"node_id" yaml:"node_id"`
	GenTxsDir  string          `json:"gentxs_dir" yaml:"gentxs_dir"`
	AppMessage json.RawMessage `json:"app_message" yaml:"app_message"`
}

func newPrintInfo(moniker, chainID, nodeID, genTxsDir string,
	appMessage json.RawMessage) printInfo {

	return printInfo{
		Moniker:    moniker,
		ChainID:    chainID,
		NodeID:     nodeID,
		GenTxsDir:  genTxsDir,
		AppMessage: appMessage,
	}
}

func displayInfo(cdc *codec.Codec, info printInfo) error {
	out, err := codec.MarshalJSONIndent(cdc, info)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(os.Stderr, "%s\n", string(sdk.MustSortJSON(out)))
	return err
}

// InitCmd returns a command that initializes all files needed for Tendermint
// and the respective application.
func CustomInitCmd(ctx *server.Context, cdc *codec.Codec, mbm module.BasicManager,
	defaultNodeHome string) *cobra.Command { // nolint: golint
	cmd := &cobra.Command{
		Use:   "init [moniker]",
		Short: "Initialize private validator, p2p, genesis, and application configuration files",
		Long:  `Initialize validators's and node's configuration files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			config := ctx.Config
			config.SetRoot(viper.GetString(cli.HomeFlag))

			chainID := viper.GetString(flags.FlagChainID)
			if chainID == "" {
				chainID = fmt.Sprintf("test-chain-%v", tmrand.Str(6))
			}

			nodeID, _, err := genutil.InitializeNodeValidatorFiles(config)
			if err != nil {
				return err
			}

			config.Moniker = args[0]

			genFile := config.GenesisFile()
			if !viper.GetBool(flagOverwrite) && tmos.FileExists(genFile) {
				return fmt.Errorf("genesis.json file already exists: %v", genFile)
			}
			appStateS := mbm.DefaultGenesis()
			stakingGenState := GetGenesisStakingStateFromAppState(cdc, appStateS)
			stakingGenState.Params.BondDenom = app.BaseCoinUnit

			stakingGenStateBz, err := cdc.MarshalJSON(stakingGenState)
			if err != nil {
				return fmt.Errorf("failed to marshal staking genesis state: %w", err)
			}

			appStateS[staking.ModuleName] = stakingGenStateBz

			govGenState := GetGenesisGovStateFromAppState(cdc, appStateS)
			govGenState.DepositParams.MinDeposit[0].Denom = app.BaseCoinUnit

			govGenStateBz, err := cdc.MarshalJSON(govGenState)
			if err != nil {
				return fmt.Errorf("failed to marshal gov genesis state: %w", err)
			}

			appStateS[gov.ModuleName] = govGenStateBz

			appState, err := codec.MarshalJSONIndent(cdc, appStateS)
			if err != nil {
				return errors.Wrap(err, "Failed to marshall default genesis state")
			}

			genDoc := &types.GenesisDoc{}
			if _, err2 := os.Stat(genFile); err2 != nil {
				if !os.IsNotExist(err) {
					return err2
				}
			} else {
				genDoc, err2 = types.GenesisDocFromFile(genFile)
				if err2 != nil {
					return errors.Wrap(err2, "Failed to read genesis doc from file")
				}
			}

			genDoc.ChainID = chainID
			genDoc.Validators = nil
			genDoc.AppState = appState
			if err = genutil.ExportGenesisFile(genDoc, genFile); err != nil {
				return errors.Wrap(err, "Failed to export gensis file")
			}

			toPrint := newPrintInfo(config.Moniker, chainID, nodeID, "", appState)

			cfg.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
			return displayInfo(cdc, toPrint)
		},
	}

	cmd.Flags().String(cli.HomeFlag, defaultNodeHome, "node's home directory")
	cmd.Flags().BoolP(flagOverwrite, "o", false, "overwrite the genesis.json file")
	cmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")

	return cmd
}
