package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tmcfg "github.com/cometbft/cometbft/config"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"
	rosettaCmd "github.com/cosmos/rosetta/cmd"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/app/params"
	"github.com/crypto-org-chain/chain-main/v8/config"
	chainmaincli "github.com/crypto-org-chain/chain-main/v8/x/chainmain/client/cli"
	memiavlcfg "github.com/crypto-org-chain/cronos/store/config"
	"github.com/imdario/mergo"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	confixcmd "cosmossdk.io/tools/confix/cmd"

	"github.com/cosmos/cosmos-sdk/client"
	clientcfg "github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

const EnvPrefix = "CRO"

// NewRootCmd creates a new root command for chain-maind. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, params.EncodingConfig) {
	config.SetConfig()

	tempApp := app.New(
		log.NewNopLogger(), dbm.NewMemDB(), nil, true,
		simtestutil.NewAppOptionsWithFlagHome(app.DefaultNodeHome),
	)
	encodingConfig := tempApp.EncodingConfig()
	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper(EnvPrefix)

	rootCmd := &cobra.Command{
		Use:   "chain-maind",
		Short: "Cronos.org Chain app",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}
			initClientCtx, err = clientcfg.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}
			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			customAppTemplate, customAppConfig := initAppConfig()

			return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, tmcfg.DefaultConfig())
		},
	}

	initRootCmd(rootCmd, encodingConfig, tempApp.BasicModuleManager)

	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, _ = clientcfg.ReadDefaultValuesFromDefaultClientConfig(initClientCtx)
	// autoCliOpts.Keyring, _ = keyring.NewAutoCLIKeyring(initClientCtx.Keyring)
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd, encodingConfig
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	// The following code snippet is just for reference.

	type CustomAppConfig struct {
		serverconfig.Config

		MemIAVL memiavlcfg.MemIAVLConfig `mapstructure:"memiavl"`
	}

	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	srvCfg := serverconfig.DefaultConfig()
	srvCfg.GRPC.Address = "127.0.0.1:9090"

	customAppConfig := CustomAppConfig{
		Config:  *srvCfg,
		MemIAVL: memiavlcfg.DefaultMemIAVLConfig(),
	}

	return serverconfig.DefaultConfigTemplate + memiavlcfg.DefaultConfigTemplate, customAppConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig params.EncodingConfig, basicManager module.BasicManager) {
	// authclient.Codec = encodingConfig.Marshaler
	cfg := sdk.GetConfig()
	cfg.Seal()

	initCmd := genutilcli.InitCmd(basicManager, app.DefaultNodeHome)
	initCmd.PreRun = func(cmd *cobra.Command, args []string) {
		serverCtx := server.GetServerContextFromCmd(cmd)
		serverCtx.Config.Consensus.TimeoutCommit = 3 * time.Second
	}
	initCmd.PostRunE = func(cmd *cobra.Command, args []string) error {
		genesisPatch := map[string]interface{}{
			"app_state": map[string]interface{}{
				"staking": map[string]interface{}{
					"params": map[string]string{
						"bond_denom": config.BaseCoinUnit,
					},
				},
				"gov": map[string]interface{}{
					"deposit_params": map[string]interface{}{
						"min_deposit": sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, govv1.DefaultMinDepositTokens)),
					},
				},
				"mint": map[string]interface{}{
					"params": map[string]string{
						"mint_denom": config.BaseCoinUnit,
					},
				},
				"bank": map[string]interface{}{
					"denom_metadata": []interface{}{
						map[string]interface{}{
							"name":        "Crypto.org Chain",
							"symbol":      "CRO",
							"description": "The native token of Crypto.org Chain.",
							"denom_units": []interface{}{
								map[string]interface{}{
									"denom":    config.BaseCoinUnit,
									"exponent": 0,
									"aliases": []interface{}{
										"carson",
									},
								},
								map[string]interface{}{
									"denom":    config.HumanCoinUnit,
									"exponent": 8,
								},
							},
							"base":    config.BaseCoinUnit,
							"display": config.HumanCoinUnit,
						},
					},
				},
				"transfer": map[string]interface{}{
					"params": map[string]bool{
						"send_enabled":    false,
						"receive_enabled": false,
					},
				},
			},
		}

		clientCtx := client.GetClientContextFromCmd(cmd)
		serverCtx := server.GetServerContextFromCmd(cmd)
		config := serverCtx.Config
		config.SetRoot(clientCtx.HomeDir)
		path := config.GenesisFile()

		cleanedPath := filepath.Clean(path)
		file, err := os.OpenFile(cleanedPath, os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Printf("Error closing file: %s\n", closeErr)
			}
		}()
		var genesis map[string]interface{}
		if decodeErr := json.NewDecoder(file).Decode(&genesis); decodeErr != nil {
			return decodeErr
		}

		if mergeErr := mergo.Merge(&genesis, genesisPatch, mergo.WithOverride); mergeErr != nil {
			return mergeErr
		}

		bz, err := json.MarshalIndent(genesis, "", "  ")
		if err != nil {
			return err
		}

		return WriteFile(cleanedPath, bz, 0o600)
	}

	rootCmd.AddCommand(
		initCmd,
		chainmaincli.AddGenesisAccountCmd(app.DefaultNodeHome),
		tmcli.NewCompletionCmd(rootCmd, true),
		chainmaincli.AddTestnetCmd(basicManager, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(newApp, app.DefaultNodeHome),
		snapshot.Cmd(newApp),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, exportAppStateAndTMValidators, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		genesisCommand(encodingConfig.TxConfig, basicManager),
		queryCommand(),
		txCommand(),
		keys.Commands(),
	)

	// add rosetta
	rootCmd.AddCommand(rosettaCmd.RosettaCommand(encodingConfig.InterfaceRegistry, encodingConfig.Marshaler))

	// versiondb changeset commands
	changeSetCmd := ChangeSetCmd()
	if changeSetCmd != nil {
		rootCmd.AddCommand(changeSetCmd)
	}
}

// genesisCommand builds genesis-related `simd genesis` command. Users may provide application specific commands as a parameter
func genesisCommand(txConfig client.TxConfig, basicManager module.BasicManager, cmds ...*cobra.Command) *cobra.Command {
	cmd := genutilcli.Commands(txConfig, basicManager, app.DefaultNodeHome)

	for _, subCmd := range cmds {
		cmd.AddCommand(subCmd)
	}
	return cmd
}

func addModuleInitFlags(startCmd *cobra.Command) {
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		server.QueryBlockCmd(),
		server.QueryBlocksCmd(),
		server.QueryBlockResultsCmd(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		chainmaincli.QueryAllTxCmd(),
	)

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	return cmd
}

// newApp is an AppCreator
func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	baseappOptions := server.DefaultBaseappOptions(appOpts)
	return app.New(logger, db, traceStore, true, appOpts, baseappOptions...)
}

// exportAppStateAndTMValidators creates a new chain app (optionally at a given height)
// and exports state.
func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB,
	traceStore io.Writer, height int64,
	forZeroHeight bool, jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	encCfg := app.MakeEncodingConfig() // Ideally, we would reuse the one created by NewRootCmd.
	encCfg.Marshaler = codec.NewProtoCodec(encCfg.InterfaceRegistry)

	var a *app.ChainApp
	if height != -1 {
		a = app.New(logger, db, traceStore, false, appOpts)

		if err := a.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		a = app.New(logger, db, traceStore, true, appOpts)
	}

	return a.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}
