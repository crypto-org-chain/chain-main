package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/crypto-com/chain-main/app/params"
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
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/crypto-com/chain-main/app"
)

// NewRootCmd creates a new root command for simd. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, params.EncodingConfig) {
	app.SetConfig()
	encodingConfig := app.MakeEncodingConfig()
	initClientCtx := client.Context{}.
		WithJSONMarshaler(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastBlock).
		WithHomeDir(app.DefaultNodeHome(appName))

	rootCmd := &cobra.Command{
		Use:   appName,
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

func convertCoin(s string) string {
	coin, err := sdk.ParseCoin(s)
	if err != nil {
		panic(err)
	}
	coin, err = sdk.ConvertCoin(coin, app.BaseCoinUnit)
	if err != nil {
		panic(err)
	}
	return coin.String()
}

func convertCoins(s string) string {
	coins, err := sdk.ParseCoins(s)
	if err != nil {
		panic(err)
	}
	for i, coin := range coins {
		coins[i], err = sdk.ConvertCoin(coin, app.BaseCoinUnit)
		if err != nil {
			panic(err)
		}
	}
	return coins.String()
}

// hack for intercepting the arguments and converting amounts
func convertDenom(args []string) {
	if len(args) >= 1 {
		switch args[0] {
		case "tx":
			if len(args) >= 4 {
				temp := args[1] + " " + args[2]
				switch temp {
				case "bank send":
					if len(args) >= 5 {
						args[5] = convertCoin(args[5])
					}
				case "staking delegate", "staking unbond":
					args[4] = convertCoin(args[4])
				case "staking redelegate":
					if len(args) >= 5 {
						args[5] = convertCoin(args[5])
					}
				}
			}
		case "gentx":
			{
				// search for --amount and take the argument next to it
				idx := -1
				for i, arg := range args {
					if arg == "--amount" {
						idx = i
					}
				}
				if idx > 0 && len(args) > idx+1 {
					args[idx+1] = convertCoin(args[idx+1])
				}
			}
		case "add-genesis-account":
			if len(args) >= 3 {
				args[2] = convertCoins(args[2])
			}
		}
	}
}

// Execute executes the root command.
func Execute(rootCmd *cobra.Command) error {
	// Create and set a client.Context on the command's Context. During the pre-run
	// of the root command, a default initialized client.Context is provided to
	// seed child command execution with values such as AccountRetriver, Keyring,
	// and a Tendermint RPC. This requires the use of a pointer reference when
	// getting and setting the client.Context. Ideally, we utilize
	// https://github.com/spf13/cobra/pull/1118.
	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &client.Context{})
	ctx = context.WithValue(ctx, server.ServerContextKey, server.NewDefaultContext())

	convertDenom(os.Args[1:])
	rootCmd.SetArgs(os.Args[1:])
	executor := tmcli.PrepareBaseCmd(rootCmd, "", app.DefaultNodeHome(appName))
	return executor.ExecuteContext(ctx)
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig params.EncodingConfig) {
	authclient.Codec = encodingConfig.Marshaler

	initCmd := genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome(appName))
	oldRunE := initCmd.RunE
	initCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := oldRunE(cmd, args); err != nil {
			return err
		}
		genesisPatch := map[string]interface{}{
			"app_state": map[string]interface{}{
				"staking": map[string]interface{}{
					"params": map[string]string{
						"bond_denom": app.BaseCoinUnit,
					},
				},
				"gov": map[string]interface{}{
					"deposit_params": map[string]interface{}{
						"min_deposit": sdk.NewCoins(sdk.NewCoin(app.BaseCoinUnit, govtypes.DefaultMinDepositTokens)),
					},
				},
				"mint": map[string]interface{}{
					"params": map[string]string{
						"mint_denom": app.BaseCoinUnit,
					},
				},
			},
		}

		clientCtx := client.GetClientContextFromCmd(cmd)
		serverCtx := server.GetServerContextFromCmd(cmd)
		config := serverCtx.Config
		config.SetRoot(clientCtx.HomeDir)
		path := config.GenesisFile()

		file, err := os.OpenFile(path, os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
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
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome(appName)),
		genutilcli.MigrateGenesisCmd(),
		genutilcli.GenTxCmd(app.ModuleBasics, encodingConfig.TxConfig,
			banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome(appName)),
		genutilcli.ValidateGenesisCmd(app.ModuleBasics, encodingConfig.TxConfig),
		AddGenesisAccountCmd(app.DefaultNodeHome(appName)),
		tmcli.NewCompletionCmd(rootCmd, true),
		debug.Cmd(),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome(appName), newApp, exportAppStateAndTMValidators)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		rpc.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(app.DefaultNodeHome(appName)),
	)
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
	)

	app.ModuleBasics.AddTxCommands(cmd)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func newApp(logger log.Logger, db dbm.DB,
	traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
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

	return app.New(
		appName, logger, db, traceStore, true, skipUpgradeHeights,
		cast.ToString(appOpts.Get(flags.FlagHome)),
		cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod)),
		app.MakeEncodingConfig(), // Ideally, we would reuse the one created by NewRootCmd.
		baseapp.SetPruning(pruningOpts),
		baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(server.FlagMinGasPrices))),
		baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(server.FlagHaltHeight))),
		baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(server.FlagHaltTime))),
		baseapp.SetInterBlockCache(cache),
		baseapp.SetTrace(cast.ToBool(appOpts.Get(server.FlagTrace))),
		baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(server.FlagIndexEvents))),
	)
}

func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailWhiteList []string,
) (servertypes.ExportedApp, error) {
	encCfg := app.MakeEncodingConfig() // Ideally, we would reuse the one created by NewRootCmd.
	encCfg.Marshaler = codec.NewProtoCodec(encCfg.InterfaceRegistry)
	var a *app.ChainApp
	if height != -1 {
		a = app.New(appName, logger, db, traceStore, false, map[int64]bool{}, "", uint(1), encCfg)

		if err := a.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		a = app.New(appName, logger, db, traceStore, true, map[int64]bool{}, "", uint(1), encCfg)
	}

	return a.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
}
