package main

// DONTCOVER

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/crypto-com/chain-main/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tmconfig "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	tos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	keyring "github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/cosmos-sdk/server"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authexported "github.com/cosmos/cosmos-sdk/x/auth/exported"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/staking"
)

var (
	flagNodeDirPrefix     = "node-dir-prefix"
	flagNumValidators     = "v"
	flagOutputDir         = "output-dir"
	flagNodeDaemonHome    = "node-daemon-home"
	flagNodeCLIHome       = "node-cli-home"
	flagStartingIPAddress = "starting-ip-address"
	flagAmount            = "amount"
	flagStakingAmount     = "staking-amount"
)

// get cmd to initialize all files for tendermint testnet and application
func testnetCmd(ctx *server.Context, cdc *codec.Codec,
	mbm module.BasicManager, genAccIterator genutiltypes.GenesisAccountsIterator,
) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Initialize files for a chain-maind testnet",
		Long: `testnet will create "v" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).

Note, strict routability for addresses is turned off in the config file.

Example:
	chain-maind testnet --v 4 --output-dir ./output --starting-ip-address 192.168.10.2
	`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config := ctx.Config

			outputDir := viper.GetString(flagOutputDir)
			chainID := viper.GetString(flags.FlagChainID)
			minGasPrices := viper.GetString(server.FlagMinGasPrices)
			nodeDirPrefix := viper.GetString(flagNodeDirPrefix)
			nodeDaemonHome := viper.GetString(flagNodeDaemonHome)
			nodeCLIHome := viper.GetString(flagNodeCLIHome)
			startingIPAddress := viper.GetString(flagStartingIPAddress)
			numValidators := viper.GetInt(flagNumValidators)
			amount := viper.GetString(flagAmount)
			stakingAmount := viper.GetString(flagStakingAmount)

			return InitTestnet(cmd, config, cdc, mbm, genAccIterator, outputDir, chainID,
				minGasPrices, nodeDirPrefix, nodeDaemonHome, nodeCLIHome, startingIPAddress, amount, stakingAmount, numValidators)
		},
	}

	cmd.Flags().Int(flagNumValidators, 4,
		"Number of validators to initialize the testnet with")
	cmd.Flags().StringP(flagOutputDir, "o", "./mytestnet",
		"Directory to store initialization data for the testnet")
	cmd.Flags().String(flagNodeDirPrefix, "node",
		"Prefix the directory name for each node with (node results in node0, node1, ...)")
	cmd.Flags().String(flagNodeDaemonHome, ".chainmaind",
		"Home directory of the node's daemon configuration")
	cmd.Flags().String(flagNodeCLIHome, ".chainmaincli",
		"Home directory of the node's cli configuration")
	cmd.Flags().String(flagStartingIPAddress, "192.168.0.1",
		`Starting IP address (192.168.0.1 results in persistent peers list
ID0@192.168.0.1:46656, ID1@192.168.0.2:46656, ...)`)
	cmd.Flags().String(
		flags.FlagChainID, "cro-test", "genesis file chain-id")
	cmd.Flags().String(
		server.FlagMinGasPrices, "",
		"Minimum gas prices to accept for transactions; All fees in a tx must meet this minimum (e.g. 0.000001cro,1basecro)")
	cmd.Flags().String(flagAmount, "200000000cro", "amount of coins for accounts")
	cmd.Flags().String(flagStakingAmount, "", "amount of coins for staking (default half of account amount)")
	cmd.Flags().String(flagVestingAmt, "", "amount of coins for vesting accounts")
	cmd.Flags().Uint64(flagVestingStart, 0, "schedule start time (unix epoch) for vesting accounts")
	cmd.Flags().Uint64(flagVestingEnd, 0, "schedule end time (unix epoch) for vesting accounts")
	return cmd
}

const nodeDirPerm = 0755

var (
	accs     []authexported.GenesisAccount
	genFiles []string
)

// Initialize the testnet
func InitTestnet(cmd *cobra.Command, config *tmconfig.Config, cdc *codec.Codec,
	mbm module.BasicManager, genAccIterator genutiltypes.GenesisAccountsIterator,
	outputDir, chainID, minGasPrices, nodeDirPrefix, nodeDaemonHome,
	nodeCLIHome, startingIPAddress, amount, stakingAmount string, numValidators int) error {

	monikers := make([]string, numValidators)
	nodeIDs := make([]string, numValidators)
	valPubKeys := make([]crypto.PubKey, numValidators)

	chainmainConfig := srvconfig.DefaultConfig()
	chainmainConfig.MinGasPrices = minGasPrices

	coins, err := sdk.ParseCoins(amount)
	if err != nil {
		return fmt.Errorf("failed to parse coins: %w", err)
	}

	for i := 0; i < len(coins); i++ {
		// nolint: govet
		coin, err := sdk.ConvertCoin(coins[i], app.BaseCoinUnit)
		if err != nil {
			return fmt.Errorf("failed to convert coins: %w", err)
		}
		coins[i] = coin
	}

	stakingCoin, err := parseStakingCoin(coins, stakingAmount)
	if err != nil {
		return err
	}

	vestingStart := viper.GetInt64(flagVestingStart)
	vestingEnd := viper.GetInt64(flagVestingEnd)
	vestingAmt, err := sdk.ParseCoins(viper.GetString(flagVestingAmt))
	if err != nil {
		return fmt.Errorf("failed to parse vesting amount: %w", err)
	}

	for i := 0; i < len(vestingAmt); i++ {
		// nolint: govet
		coin, err := sdk.ConvertCoin(vestingAmt[i], app.BaseCoinUnit)
		if err != nil {
			return fmt.Errorf("failed to convert coins: %w", err)
		}
		vestingAmt[i] = coin
	}

	// generate private keys, node IDs, and initial transactions
	for i := 0; i < numValidators; i++ {
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
		nodeDir := filepath.Join(outputDir, nodeDirName, nodeDaemonHome)
		clientDir := filepath.Join(outputDir, nodeDirName, nodeCLIHome)
		gentxsDir := filepath.Join(outputDir, "gentxs")

		config.SetRoot(nodeDir)
		config.RPC.ListenAddress = "tcp://0.0.0.0:26657"

		if err = os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		if err = os.MkdirAll(clientDir, nodeDirPerm); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		monikers = append(monikers, nodeDirName)
		config.Moniker = nodeDirName

		ip, ipErr := getIP(i, startingIPAddress)
		if ipErr != nil {
			_ = os.RemoveAll(outputDir)
			return ipErr
		}

		nodeIDs[i], valPubKeys[i], err = genutil.InitializeNodeValidatorFiles(config)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		memo := fmt.Sprintf("%s@%s:26656", nodeIDs[i], ip)
		genFiles = append(genFiles, config.GenesisFile())

		buf := bufio.NewReader(cmd.InOrStdin())
		prompt := fmt.Sprintf(
			"Password for account '%s' (default %s):", nodeDirName, keys.DefaultKeyPass,
		)

		keyPass, kpErr := input.GetPassword(prompt, buf)
		if kpErr != nil && keyPass != "" {
			// An error was returned that either failed to read the password from
			// STDIN or the given password is not empty but failed to meet minimum
			// length requirements.
			return kpErr
		}

		if keyPass == "" {
			keyPass = keys.DefaultKeyPass
		}
		kb, ringErr := keyring.NewKeyring(sdk.KeyringServiceName(), keyring.BackendTest, clientDir, buf)
		if ringErr != nil {
			return ringErr
		}

		keyInfo, secret, mErr := kb.CreateMnemonic(nodeDirName, keyring.English, keyPass, keyring.Secp256k1)
		if mErr != nil {
			_ = os.RemoveAll(outputDir)
			return mErr
		}

		addr := keyInfo.GetAddress()
		info := map[string]string{"secret": secret, "addr": addr.String()}

		cliPrint, jsonErr := json.Marshal(info)
		if jsonErr != nil {
			return jsonErr
		}

		// save private key seed words
		if err = writeFile(fmt.Sprintf("%v.json", "key_seed"), clientDir, cliPrint); err != nil {
			return err
		}

		// create concrete account type based on input parameters
		var genAccount authexported.GenesisAccount
		baseAccount := auth.NewBaseAccount(addr, coins.Sort(), nil, 0, 0)

		if !vestingAmt.IsZero() {
			// nolint: govet
			baseVestingAccount, err := authvesting.NewBaseVestingAccount(baseAccount, vestingAmt.Sort(), vestingEnd)
			if err != nil {
				return fmt.Errorf("failed to create base vesting account: %w", err)
			}

			switch {
			case vestingStart != 0 && vestingEnd != 0:
				genAccount = authvesting.NewContinuousVestingAccountRaw(baseVestingAccount, vestingStart)

			case vestingEnd != 0:
				genAccount = authvesting.NewDelayedVestingAccountRaw(baseVestingAccount)

			default:
				return errors.New("invalid vesting parameters; must supply start and end time or end time")
			}
		} else {
			genAccount = baseAccount
		}

		// nolint: govet
		if err := genAccount.Validate(); err != nil {
			return fmt.Errorf("failed to validate new genesis account: %w", err)
		}
		accs = append(accs, genAccount)

		msg := staking.NewMsgCreateValidator(
			sdk.ValAddress(addr),
			valPubKeys[i],
			stakingCoin,
			staking.NewDescription(nodeDirName, "", "", "", ""),
			staking.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.NewDecWithPrec(2, 1), sdk.NewDecWithPrec(1, 2)),
			sdk.OneInt(),
		)

		tx := auth.NewStdTx([]sdk.Msg{msg}, auth.StdFee{}, []auth.StdSignature{}, memo)
		txBldr := auth.NewTxBuilderFromCLI(buf).WithChainID(chainID).WithMemo(memo).WithKeybase(kb)

		signedTx, txErr := txBldr.SignStdTx(nodeDirName, keys.DefaultKeyPass, tx, false)
		if txErr != nil {
			_ = os.RemoveAll(outputDir)
			return txErr
		}

		txBytes, cdcErr := cdc.MarshalJSON(signedTx)
		if cdcErr != nil {
			_ = os.RemoveAll(outputDir)
			return cdcErr
		}

		// gather gentxs folder
		if err = writeFile(fmt.Sprintf("%v.json", nodeDirName), gentxsDir, txBytes); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		chainmainConfigFilePath := filepath.Join(nodeDir, "config/app.toml")
		srvconfig.WriteConfigFile(chainmainConfigFilePath, chainmainConfig)
	}

	if err = initGenFiles(cdc, mbm, chainID, accs, genFiles, numValidators); err != nil {
		return err
	}

	err = collectGenFiles(
		cdc, config, chainID, monikers, nodeIDs, valPubKeys, numValidators,
		outputDir, nodeDirPrefix, nodeDaemonHome, genAccIterator,
	)
	if err != nil {
		return err
	}

	cmd.PrintErrf("Successfully initialized %d node directories\n", numValidators)
	return nil
}

func initGenFiles(cdc *codec.Codec, mbm module.BasicManager, chainID string,
	accs []authexported.GenesisAccount, genFiles []string, numValidators int) error {

	appGenState := mbm.DefaultGenesis()

	stakingGenState := GetGenesisStakingStateFromAppState(cdc, appGenState)
	stakingGenState.Params.BondDenom = app.BaseCoinUnit

	stakingGenStateBz, err := cdc.MarshalJSON(stakingGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal staking genesis state: %w", err)
	}

	appGenState[staking.ModuleName] = stakingGenStateBz

	govGenState := GetGenesisGovStateFromAppState(cdc, appGenState)
	govGenState.DepositParams.MinDeposit[0].Denom = app.BaseCoinUnit

	govGenStateBz, err := cdc.MarshalJSON(govGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal gov genesis state: %w", err)
	}

	appGenState[gov.ModuleName] = govGenStateBz

	// set the accounts in the genesis state
	authGenState := auth.GetGenesisStateFromAppState(cdc, appGenState)
	// Add the new account to the set of genesis accounts and sanitize the
	// accounts afterwards.
	for _, acc := range accs {
		authGenState.Accounts = append(authGenState.Accounts, acc)
	}
	authGenState.Accounts = auth.SanitizeGenesisAccounts(authGenState.Accounts)
	authGenStateBz, err := cdc.MarshalJSON(authGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal auth genesis state: %w", err)
	}
	appGenState[auth.ModuleName] = authGenStateBz

	appGenStateJSON, err := codec.MarshalJSONIndent(cdc, appGenState)
	if err != nil {
		return err
	}

	genDoc := types.GenesisDoc{
		ChainID:    chainID,
		AppState:   appGenStateJSON,
		Validators: nil,
	}

	// generate empty genesis files for each validator and save
	for i := 0; i < numValidators; i++ {
		if err := genDoc.SaveAs(genFiles[i]); err != nil {
			return err
		}
	}
	return nil
}

func collectGenFiles(
	cdc *codec.Codec, config *tmconfig.Config, chainID string,
	monikers, nodeIDs []string, valPubKeys []crypto.PubKey,
	numValidators int, outputDir, nodeDirPrefix, nodeDaemonHome string,
	genAccIterator genutiltypes.GenesisAccountsIterator) error {

	var appState json.RawMessage
	genTime := tmtime.Now()

	for i := 0; i < numValidators; i++ {
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
		nodeDir := filepath.Join(outputDir, nodeDirName, nodeDaemonHome)
		gentxsDir := filepath.Join(outputDir, "gentxs")
		moniker := monikers[i]
		config.Moniker = nodeDirName

		config.SetRoot(nodeDir)

		nodeID, valPubKey := nodeIDs[i], valPubKeys[i]
		initCfg := genutil.NewInitConfig(chainID, gentxsDir, moniker, nodeID, valPubKey)

		genDoc, err := types.GenesisDocFromFile(config.GenesisFile())
		if err != nil {
			return err
		}

		nodeAppState, err := genutil.GenAppStateFromConfig(cdc, config, initCfg, *genDoc, genAccIterator)
		if err != nil {
			return err
		}

		if appState == nil {
			// set the canonical application state (they should not differ)
			appState = nodeAppState
		}

		genFile := config.GenesisFile()

		// overwrite each validator's genesis file to have a canonical genesis time
		if err := genutil.ExportGenesisFileWithTime(genFile, chainID, nil, appState, genTime); err != nil {
			return err
		}
	}

	return nil
}

func getIP(i int, startingIPAddr string) (ip string, err error) {
	if len(startingIPAddr) == 0 {
		ip, err = server.ExternalIP()
		if err != nil {
			return "", err
		}
		return ip, nil
	}
	return calculateIP(startingIPAddr, i)
}

func calculateIP(ip string, i int) (string, error) {
	ipv4 := net.ParseIP(ip).To4()
	if ipv4 == nil {
		return "", fmt.Errorf("%v: non ipv4 address", ip)
	}

	for j := 0; j < i; j++ {
		ipv4[3]++
	}

	return ipv4.String(), nil
}

func writeFile(name string, dir string, contents []byte) error {
	writePath := filepath.Join(dir)
	file := filepath.Join(writePath, name)

	err := tos.EnsureDir(writePath, 0700)
	if err != nil {
		return err
	}

	err = tos.WriteFile(file, contents, 0600)
	if err != nil {
		return err
	}

	return nil
}

func parseStakingCoin(coins sdk.Coins, stakingAmount string) (sdk.Coin, error) {
	stakingCoin := sdk.Coin{
		Denom:  app.BaseCoinUnit,
		Amount: sdk.ZeroInt(),
	}
	// return half amount of coins from account and error if any
	if stakingAmount == "" {
		for _, coin := range coins {
			halfCoin := sdk.Coin{
				Denom:  app.BaseCoinUnit,
				Amount: coin.Amount.Quo(sdk.NewInt(2)),
			}
			stakingCoin = stakingCoin.Add(halfCoin)
		}
	} else {
		coin, err := sdk.ParseCoin(stakingAmount)
		if err != nil {
			return coin, fmt.Errorf("failed to parse staking coin: %w", err)
		}
		coin, err = sdk.ConvertCoin(stakingCoin, app.BaseCoinUnit)
		if err != nil {
			return coin, fmt.Errorf("failed to convert staking coin: %w", err)
		}
		stakingCoin = coin
	}
	return stakingCoin, nil
}
