package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	conf "github.com/cosmos/cosmos-sdk/client/config"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	"github.com/imdario/mergo"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	tmcfg "github.com/tendermint/tendermint/config"
	tmcli "github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/snapshots"
	snapshottypes "github.com/cosmos/cosmos-sdk/snapshots/types"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/chain-main/v4/app/params"
	"github.com/crypto-org-chain/chain-main/v4/config"
	chainmaincli "github.com/crypto-org-chain/chain-main/v4/x/chainmain/client/cli"
)

const EnvPrefix = "CRO"

// NewRootCmd creates a new root command for chain-maind. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, params.EncodingConfig) {
	config.SetConfig()
	encodingConfig := app.MakeEncodingConfig()
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
		Short: "Crypto.org Chain app",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			// nolint: govet
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}
			// nolint: govet
			initClientCtx, err = conf.ReadFromClientConfig(initClientCtx)
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

	initRootCmd(rootCmd, encodingConfig)

	return rootCmd, encodingConfig
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	// The following code snippet is just for reference.

	type CustomAppConfig struct {
		serverconfig.Config
	}

	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	srvCfg := serverconfig.DefaultConfig()
	srvCfg.GRPC.Address = "127.0.0.1:9090"
	srvCfg.GRPCWeb.Address = "127.0.0.1:9091"

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
	}

	return serverconfig.DefaultConfigTemplate, customAppConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig params.EncodingConfig) {
	// authclient.Codec = encodingConfig.Marshaler
	cfg := sdk.GetConfig()
	cfg.Seal()

	initCmd := genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome)
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
		// nolint: gosec
		file, err := os.OpenFile(cleanedPath, os.O_RDWR, 0600)
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

		return WriteFile(cleanedPath, bz, 0600)
	}

	rootCmd.AddCommand(
		initCmd,
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome),
		genutilcli.MigrateGenesisCmd(),
		genutilcli.GenTxCmd(app.ModuleBasics, encodingConfig.TxConfig, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome),
		genutilcli.ValidateGenesisCmd(app.ModuleBasics),
		chainmaincli.AddGenesisAccountCmd(app.DefaultNodeHome),
		tmcli.NewCompletionCmd(rootCmd, true),
		chainmaincli.AddTestnetCmd(app.ModuleBasics, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
		conf.Cmd(),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, exportAppStateAndTMValidators, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		rpc.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(app.DefaultNodeHome),
	)

	// add rosetta
	rootCmd.AddCommand(server.RosettaCommand(encodingConfig.InterfaceRegistry, encodingConfig.Marshaler))

	// versiondb changeset commands
	changeSetCmd := ChangeSetCmd()
	if changeSetCmd != nil {
		rootCmd.AddCommand(changeSetCmd)
	}
}

func addModuleInitFlags(startCmd *cobra.Command) {
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetAccountCmd(),
		rpc.ValidatorCommand(),
		rpc.BlockCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		chainmaincli.QueryAllTxCmd(),
	)

	app.ModuleBasics.AddQueryCommands(cmd)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         true,
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
	)

	app.ModuleBasics.AddTxCommands(cmd)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

// newApp is an AppCreator
func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	var cache sdk.MultiStorePersistentCache

	if cast.ToBool(appOpts.Get(server.FlagInterBlockCache)) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	pruningOpts, err := server.GetPruningOptionsFromFlags(appOpts)
	if err != nil {
		panic(err)
	}

	snapshotDir := filepath.Join(cast.ToString(appOpts.Get(flags.FlagHome)), "data", "snapshots")
	err = os.MkdirAll(snapshotDir, os.ModePerm)
	if err != nil {
		panic(err)
	}

	snapshotDB, err := dbm.NewDB("metadata", server.GetAppDBBackend(appOpts), snapshotDir)
	if err != nil {
		panic(err)
	}
	snapshotStore, err := snapshots.NewStore(snapshotDB, snapshotDir)
	if err != nil {
		panic(err)
	}

	snapshotOptions := snapshottypes.NewSnapshotOptions(
		cast.ToUint64(appOpts.Get(server.FlagStateSyncSnapshotInterval)),
		cast.ToUint32(appOpts.Get(server.FlagStateSyncSnapshotKeepRecent)),
	)

	return app.New(
		logger, db, traceStore, true, skipUpgradeHeights,
		cast.ToString(appOpts.Get(flags.FlagHome)),
		cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod)),
		app.MakeEncodingConfig(), // Ideally, we would reuse the one created by NewRootCmd.
		appOpts,
		baseapp.SetPruning(pruningOpts),
		baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(server.FlagMinGasPrices))),
		baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(server.FlagHaltHeight))),
		baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(server.FlagHaltTime))),
		baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(server.FlagMinRetainBlocks))),
		baseapp.SetInterBlockCache(cache),
		baseapp.SetTrace(cast.ToBool(appOpts.Get(server.FlagTrace))),
		baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(server.FlagIndexEvents))),
		baseapp.SetSnapshot(snapshotStore, snapshotOptions),
		baseapp.SetIAVLCacheSize(cast.ToInt(appOpts.Get(server.FlagIAVLCacheSize))),
		baseapp.SetIAVLDisableFastNode(cast.ToBool(appOpts.Get(server.FlagDisableIAVLFastNode))),
	)
}

// exportAppStateAndTMValidators creates a new chain app (optionally at a given height)
// and exports state.
func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailAllowedAddrs []string,
	appOpts servertypes.AppOptions) (servertypes.ExportedApp, error) {

	encCfg := app.MakeEncodingConfig() // Ideally, we would reuse the one created by NewRootCmd.
	encCfg.Marshaler = codec.NewProtoCodec(encCfg.InterfaceRegistry)

	var a *app.ChainApp
	if height != -1 {
		a = app.New(logger, db, traceStore, false, map[int64]bool{}, "", uint(1), encCfg, appOpts)

		if err := a.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		a = app.New(logger, db, traceStore, true, map[int64]bool{}, "", uint(1), encCfg, appOpts)
	}

	return a.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs)
}
