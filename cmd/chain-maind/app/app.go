package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/imdario/mergo"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
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
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingcli "github.com/cosmos/cosmos-sdk/x/auth/vesting/client/cli"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/crypto-com/chain-main/app"
	"github.com/crypto-com/chain-main/app/params"
	"github.com/crypto-com/chain-main/config"
	chainmaincli "github.com/crypto-com/chain-main/x/chainmain/client/cli"
)

// NewRootCmd creates a new root command for chain-maind. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, params.EncodingConfig) {
	config.SetConfig()
	encodingConfig := app.MakeEncodingConfig()
	initClientCtx := client.Context{}.
		WithJSONMarshaler(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastBlock).
		WithHomeDir(app.DefaultNodeHome)

	rootCmd := &cobra.Command{
		Use:   "chain-maind",
		Short: "Crypto.com Chain app",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			return server.InterceptConfigsPreRunHandler(cmd)
		},
	}

	initRootCmd(rootCmd, encodingConfig)

	return rootCmd, encodingConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig params.EncodingConfig) {
	authclient.Codec = encodingConfig.Marshaler

	initCmd := genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome)
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
						"min_deposit": sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, govtypes.DefaultMinDepositTokens)),
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
							"description": "The native token of Crypto.com app.",
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
		if err := json.NewDecoder(file).Decode(&genesis); err != nil {
			return err
		}

		if err := mergo.Merge(&genesis, genesisPatch, mergo.WithOverride); err != nil {
			return err
		}
		if err := file.Truncate(0); err != nil {
			return err
		}
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}
		return json.NewEncoder(file).Encode(&genesis)
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
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, exportAppStateAndTMValidators, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		rpc.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(app.DefaultNodeHome),
	)
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
		authcmd.GetValidateSignaturesCommand(),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		flags.LineBreak,
		vestingcli.GetTxCmd(),
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
	snapshotDB, err := sdk.NewLevelDB("metadata", snapshotDir)
	if err != nil {
		panic(err)
	}
	snapshotStore, err := snapshots.NewStore(snapshotDB, snapshotDir)
	if err != nil {
		panic(err)
	}

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
		baseapp.SetSnapshotStore(snapshotStore),
		baseapp.SetSnapshotInterval(cast.ToUint64(appOpts.Get(server.FlagStateSyncSnapshotInterval))),
		baseapp.SetSnapshotKeepRecent(cast.ToUint32(appOpts.Get(server.FlagStateSyncSnapshotKeepRecent))),
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
