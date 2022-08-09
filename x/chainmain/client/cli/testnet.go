package cli

// DONTCOVER

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	sdkmath "cosmossdk.io/math"
	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/spf13/cobra"
	tmconfig "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var (
	flagNodeDirPrefix     = "node-dir-prefix"
	flagNumValidators     = "v"
	flagOutputDir         = "output-dir"
	flagNodeDaemonHome    = "node-daemon-home"
	flagStartingIPAddress = "starting-ip-address"
	flagAmount            = "amount"
	flagStakingAmount     = "staking-amount"
	flagUnbondingTime     = "unbonding-time"
)

// get cmd to initialize all files for tendermint testnet and application
func AddTestnetCmd(
	mbm module.BasicManager,
	genBalIterator banktypes.GenesisBalancesIterator,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Initialize files for a simapp testnet",
		Long: `testnet will create "v" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).
Note, strict routability for addresses is turned off in the config file.
Example:
	chain-maind testnet --v 4 --output-dir ./output --starting-ip-address 192.168.10.2
	`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			outputDir, outputDirErr := cmd.Flags().GetString(flagOutputDir)
			if outputDirErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagOutputDir, outputDirErr)
			}
			keyringBackend, keyringBackendErr := cmd.Flags().GetString(flags.FlagKeyringBackend)
			if keyringBackendErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flags.FlagKeyringBackend, keyringBackendErr)
			}
			chainID, chainIDErr := cmd.Flags().GetString(flags.FlagChainID)
			if chainIDErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flags.FlagChainID, chainIDErr)
			}
			minGasPrices, minGasPricesErr := cmd.Flags().GetString(server.FlagMinGasPrices)
			if minGasPricesErr != nil {
				return fmt.Errorf("failed to parse %v: %w", server.FlagMinGasPrices, minGasPricesErr)
			}
			nodeDirPrefix, nodeDirPrefixErr := cmd.Flags().GetString(flagNodeDirPrefix)
			if nodeDirPrefixErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagNodeDirPrefix, nodeDirPrefixErr)
			}
			nodeDaemonHome, nodeDaemonHomeErr := cmd.Flags().GetString(flagNodeDaemonHome)
			if nodeDaemonHomeErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagNodeDaemonHome, nodeDaemonHomeErr)
			}
			startingIPAddress, startingIPAddressErr := cmd.Flags().GetString(flagStartingIPAddress)
			if startingIPAddressErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagStartingIPAddress, startingIPAddressErr)
			}
			numValidators, numValidatorsErr := cmd.Flags().GetUint(flagNumValidators)
			if numValidatorsErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagNumValidators, numValidatorsErr)
			}
			algo, algoErr := cmd.Flags().GetString(flags.FlagKeyAlgorithm)
			if algoErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flags.FlagKeyAlgorithm, algoErr)
			}
			amount, amountErr := cmd.Flags().GetString(flagAmount)
			if amountErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagAmount, amountErr)
			}
			stakingAmount, stakingAmountErr := cmd.Flags().GetString(flagStakingAmount)
			if stakingAmountErr != nil {
				return fmt.Errorf("failed to parse %v: %w", flagStakingAmount, stakingAmountErr)
			}

			return InitTestnet(
				clientCtx, cmd, config, mbm, genBalIterator, outputDir, chainID, minGasPrices,
				nodeDirPrefix, nodeDaemonHome, startingIPAddress, keyringBackend, algo, amount, stakingAmount, int(numValidators),
			)
		},
	}

	cmd.Flags().Uint(flagNumValidators, 4, "Number of validators to initialize the testnet with")
	cmd.Flags().StringP(flagOutputDir, "o", "./mytestnet", "Directory to store initialization data for the testnet")
	cmd.Flags().String(flagNodeDirPrefix,
		"node", "Prefix the directory name for each node with (node results in node0, node1, ...)")
	cmd.Flags().String(flagNodeDaemonHome, ".chain-maind", "Home directory of the node's daemon configuration")
	cmd.Flags().String(flagStartingIPAddress,
		"192.168.0.1",
		"Starting IP address (192.168.0.1 results in persistent peers list ID0@192.168.0.1:46656, ID1@192.168.0.2:46656, ...)") //nolint
	cmd.Flags().String(flags.FlagChainID, "cro-test", "genesis file chain-id")
	cmd.Flags().String(server.FlagMinGasPrices, "",
		"Minimum gas prices to accept for transactions; All fees in a tx must meet this minimum (e.g. 0.00000001cro,1basecro)") //nolint
	cmd.Flags().String(flags.FlagKeyringBackend, keyring.BackendTest, "Select keyring's backend (os|file|test)")
	cmd.Flags().String(flags.FlagKeyAlgorithm, string(hd.Secp256k1Type), "Key signing algorithm to generate keys for")
	cmd.Flags().String(flagAmount, "20000000000000000basecro", "amount of coins for accounts")
	cmd.Flags().String(flagStakingAmount, "", "amount of coins for staking (default half of account amount)")
	cmd.Flags().String(flagVestingAmt, "", "amount of coins for vesting accounts")
	cmd.Flags().Uint32(flagVestingStart, 0, "schedule start time (unix epoch) for vesting accounts")
	cmd.Flags().Uint32(flagVestingEnd, 0, "schedule end time (unix epoch) for vesting accounts")
	cmd.Flags().String(flagUnbondingTime, "1814400s", "unbonding time")

	return cmd
}

const nodeDirPerm = 0755

var (
	genAccounts []authtypes.GenesisAccount
	genBalances []banktypes.Balance
	genFiles    []string
)

// Initialize the testnet
func InitTestnet(
	clientCtx client.Context,
	cmd *cobra.Command,
	nodeConfig *tmconfig.Config,
	mbm module.BasicManager,
	genBalIterator banktypes.GenesisBalancesIterator,
	outputDir,
	chainID,
	minGasPrices,
	nodeDirPrefix,
	nodeDaemonHome,
	startingIPAddress,
	keyringBackend,
	algoStr,
	amount,
	stakingAmount string,
	numValidators int,
) error {
	nodeIDs := make([]string, numValidators)
	valPubKeys := make([]crypto.PubKey, numValidators)

	chainmainConfig := srvconfig.DefaultConfig()
	chainmainConfig.MinGasPrices = minGasPrices
	chainmainConfig.API.Enable = true
	chainmainConfig.Telemetry.Enabled = true
	chainmainConfig.Telemetry.PrometheusRetentionTime = 60
	chainmainConfig.Telemetry.EnableHostnameLabel = false
	chainmainConfig.Telemetry.GlobalLabels = [][]string{{"chain_id", chainID}}

	coins, err := sdk.ParseCoinsNormalized(amount)
	if err != nil {
		return fmt.Errorf("failed to parse coins: %w", err)
	}

	stakingCoin, err := parseStakingCoin(coins, stakingAmount)
	if err != nil {
		return err
	}

	vestingStart, errstart := cmd.Flags().GetUint32(flagVestingStart)
	if errstart != nil {
		return fmt.Errorf("failed to parse vesting start: %w", errstart)
	}
	vestingEnd, errend := cmd.Flags().GetUint32(flagVestingEnd)
	if errend != nil {
		return fmt.Errorf("failed to parse vesting end: %w", errend)
	}
	vestingAmtStr, erramt := cmd.Flags().GetString(flagVestingAmt)
	if erramt != nil {
		return fmt.Errorf("failed to parse vesting amount: %w", erramt)
	}
	vestingCoins, err := sdk.ParseCoinsNormalized(vestingAmtStr)
	if err != nil {
		return fmt.Errorf("failed to parse vesting amount: %w", err)
	}
	unbondingTimeStr, unbondingTimeStrErr := cmd.Flags().GetString(flagUnbondingTime)
	if unbondingTimeStrErr != nil {
		return fmt.Errorf("failed to parse %v: %w", flagUnbondingTime, unbondingTimeStrErr)
	}
	unbondingTime, unbondingTimeErr := time.ParseDuration(unbondingTimeStr)
	if unbondingTimeStrErr != nil {
		return fmt.Errorf("failed to parse %v: %w", flagUnbondingTime, unbondingTimeErr)
	}

	inBuf := bufio.NewReader(cmd.InOrStdin())
	// generate private keys, node IDs, and initial transactions
	for i := 0; i < numValidators; i++ {
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
		nodeDir := filepath.Join(outputDir, nodeDirName, nodeDaemonHome)
		gentxsDir := filepath.Join(outputDir, "gentxs")

		nodeConfig.SetRoot(nodeDir)
		nodeConfig.RPC.ListenAddress = "tcp://0.0.0.0:26657"

		if err = os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		nodeConfig.Moniker = nodeDirName

		ip, ipErr := getIP(i, startingIPAddress)
		if ipErr != nil {
			_ = os.RemoveAll(outputDir)
			return ipErr
		}

		nodeIDs[i], valPubKeys[i], err = genutil.InitializeNodeValidatorFiles(nodeConfig)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		memo := fmt.Sprintf("%s@%s:26656", nodeIDs[i], ip)
		genFiles = append(genFiles, nodeConfig.GenesisFile())

		kb, keyErr := keyring.New(sdk.KeyringServiceName(), keyringBackend, nodeDir, inBuf, clientCtx.Codec)
		if keyErr != nil {
			return keyErr
		}

		keyringAlgos, _ := kb.SupportedAlgorithms()
		algo, algoErr := keyring.NewSigningAlgoFromString(algoStr, keyringAlgos)
		if algoErr != nil {
			return algoErr
		}

		keyInfo, secret, saveErr := kb.NewMnemonic(nodeDirName,
			keyring.English, sdk.GetConfig().GetFullBIP44Path(), keyring.DefaultBIP39Passphrase, algo)
		if saveErr != nil {
			_ = os.RemoveAll(outputDir)
			return saveErr
		}
		var addr sdk.AccAddress
		addr, err = keyInfo.GetAddress()
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		info := map[string]string{"secret": secret, "addr": addr.String()}

		cliPrint, jErr := json.Marshal(info)
		if jErr != nil {
			return jErr
		}

		// save private key seed words
		if err = writeFile(fmt.Sprintf("%v.json", "key_seed"), nodeDir, cliPrint); err != nil {
			return err
		}

		// create concrete account type based on input parameters
		var genAccount authtypes.GenesisAccount

		genbalance := banktypes.Balance{Address: addr.String(), Coins: coins.Sort()}
		baseAccount := authtypes.NewBaseAccount(addr, nil, 0, 0)

		if !vestingCoins.IsZero() {
			baseVestingAccount := authvesting.NewBaseVestingAccount(baseAccount, vestingCoins.Sort(), int64(vestingEnd))

			if (genbalance.Coins.IsZero() && !baseVestingAccount.OriginalVesting.IsZero()) ||
				baseVestingAccount.OriginalVesting.IsAnyGT(genbalance.Coins) {
				return errors.New("vesting amount cannot be greater than total amount")
			}

			switch {
			case vestingStart != 0 && vestingEnd != 0:
				genAccount = authvesting.NewContinuousVestingAccountRaw(baseVestingAccount, int64(vestingStart))

			case vestingEnd != 0:
				genAccount = authvesting.NewDelayedVestingAccountRaw(baseVestingAccount)

			default:
				return errors.New("invalid vesting parameters; must supply start and end time or end time")
			}
		} else {
			genAccount = baseAccount
		}

		if err = genAccount.Validate(); err != nil {
			return fmt.Errorf("failed to validate new genesis account: %w", err)
		}

		genBalances = append(genBalances, genbalance)
		genAccounts = append(genAccounts, genAccount)

		createValMsg, err2 := stakingtypes.NewMsgCreateValidator(
			sdk.ValAddress(addr),
			valPubKeys[i],
			stakingCoin,
			stakingtypes.NewDescription(nodeDirName, "", "", "", ""),
			stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.NewDecWithPrec(2, 1), sdk.NewDecWithPrec(1, 2)),
			sdk.OneInt(),
		)
		if err2 != nil {
			return err2
		}

		txBuilder := clientCtx.TxConfig.NewTxBuilder()
		if err = txBuilder.SetMsgs(createValMsg); err != nil {
			return err
		}

		txBuilder.SetMemo(memo)

		txFactory := tx.Factory{}
		txFactory = txFactory.
			WithChainID(chainID).
			WithMemo(memo).
			WithKeybase(kb).
			WithTxConfig(clientCtx.TxConfig)

		if err = tx.Sign(txFactory, nodeDirName, txBuilder, true); err != nil {
			return err
		}

		txBz, txErr := clientCtx.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
		if txErr != nil {
			return txErr
		}

		if err = writeFile(fmt.Sprintf("%v.json", nodeDirName), gentxsDir, txBz); err != nil {
			return err
		}

		srvconfig.WriteConfigFile(filepath.Join(nodeDir, "config/app.toml"), chainmainConfig)
	}

	if err = initGenFiles(
		clientCtx, mbm, chainID, genAccounts, genBalances,
		genFiles, numValidators, unbondingTime); err != nil {
		return err
	}

	err = collectGenFiles(
		clientCtx, nodeConfig, chainID, nodeIDs, valPubKeys, numValidators,
		outputDir, nodeDirPrefix, nodeDaemonHome, genBalIterator,
	)
	if err != nil {
		return err
	}

	cmd.PrintErrf("Successfully initialized %d node directories\n", numValidators)
	return nil
}

func initGenFiles(
	clientCtx client.Context, mbm module.BasicManager, chainID string,
	genAccounts []authtypes.GenesisAccount, genBalances []banktypes.Balance,
	genFiles []string, numValidators int,
	unbondingTime time.Duration,
) error {
	appGenState := mbm.DefaultGenesis(clientCtx.Codec)

	// set staking param in the genesis state

	var stakingGenState stakingtypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[stakingtypes.ModuleName], &stakingGenState)
	baseDenom, err := sdk.GetBaseDenom()
	if err != nil {
		return err
	}
	stakingGenState.Params.BondDenom = baseDenom
	stakingGenState.Params.UnbondingTime = unbondingTime

	appGenState[stakingtypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&stakingGenState)

	// set gov min_deposit in the genesis state
	var govGenState govv1.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[govtypes.ModuleName], &govGenState)
	govGenState.DepositParams.MinDeposit[0].Denom = baseDenom
	appGenState[govtypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&govGenState)

	// set mint in the genesis state
	var mintGenState minttypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[minttypes.ModuleName], &mintGenState)
	mintGenState.Params.MintDenom = baseDenom
	appGenState[minttypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&mintGenState)

	// set the accounts in the genesis state
	var authGenState authtypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[authtypes.ModuleName], &authGenState)

	accounts, err := authtypes.PackAccounts(genAccounts)
	if err != nil {
		return err
	}

	authGenState.Accounts = accounts
	appGenState[authtypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&authGenState)

	// set the balances in the genesis state
	var bankGenState banktypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[banktypes.ModuleName], &bankGenState)

	bankGenState.Balances = genBalances
	appGenState[banktypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&bankGenState)

	appGenStateJSON, err := json.MarshalIndent(appGenState, "", "  ")
	if err != nil {
		return err
	}

	genDoc := tmtypes.GenesisDoc{
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
	clientCtx client.Context, nodeConfig *tmconfig.Config, chainID string,
	nodeIDs []string, valPubKeys []crypto.PubKey, numValidators int,
	outputDir, nodeDirPrefix, nodeDaemonHome string, genBalIterator banktypes.GenesisBalancesIterator,
) error {

	var appState json.RawMessage
	genTime := tmtime.Now()

	for i := 0; i < numValidators; i++ {
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
		nodeDir := filepath.Join(outputDir, nodeDirName, nodeDaemonHome)
		gentxsDir := filepath.Join(outputDir, "gentxs")
		nodeConfig.Moniker = nodeDirName

		nodeConfig.SetRoot(nodeDir)

		nodeID, valPubKey := nodeIDs[i], valPubKeys[i]
		initCfg := genutiltypes.NewInitConfig(chainID, gentxsDir, nodeID, valPubKey)

		genDoc, err := tmtypes.GenesisDocFromFile(nodeConfig.GenesisFile())
		if err != nil {
			return err
		}

		nodeAppState, err := genutil.GenAppStateFromConfig(
			clientCtx.Codec, clientCtx.TxConfig,
			nodeConfig, initCfg, *genDoc, genBalIterator)
		if err != nil {
			return err
		}

		if appState == nil {
			// set the canonical application state (they should not differ)
			appState = nodeAppState
		}

		genFile := nodeConfig.GenesisFile()

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
	file := filepath.Join(dir, name)

	err := tmos.EnsureDir(dir, 0755)
	if err != nil {
		return err
	}

	err = tmos.WriteFile(file, contents, 0600)
	if err != nil {
		return err
	}

	return nil
}

func parseStakingCoin(coins sdk.Coins, stakingAmount string) (sdk.Coin, error) {
	baseDenom, err := sdk.GetBaseDenom()
	if err != nil {
		return sdk.Coin{}, nil
	}
	stakingCoin := sdk.Coin{
		Denom:  baseDenom,
		Amount: sdk.ZeroInt(),
	}
	if stakingAmount == "" {
		stakingCoin.Amount = halfCoins(coins)
	} else {
		coins, err := sdk.ParseCoinsNormalized(stakingAmount)
		if err != nil {
			return stakingCoin, fmt.Errorf("failed to parse staking coin: %w", err)
		}
		stakingCoin = mergeCoins(coins)
	}
	return stakingCoin, nil
}

// return half amount of coins
func halfCoins(coins sdk.Coins) sdkmath.Int {
	amount := sdkmath.ZeroInt()
	for _, coin := range coins {
		amount = amount.Add(coin.Amount.Quo(sdkmath.NewInt(2)))
	}
	return amount
}

// merge coins into coin
func mergeCoins(coins sdk.Coins) sdk.Coin {
	coin := coins[0]
	for i := 1; i < len(coins); i++ {
		coin = coin.Add(coins[i])
	}
	return coin
}
