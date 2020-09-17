package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/tendermint/tendermint/crypto"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	stakingcli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
)

var (
	defaultTokens                  = sdk.TokensFromConsensusPower(100)
	defaultAmount                  = defaultTokens.String() + sdk.DefaultBondDenom
	defaultCommissionRate          = "0.1"
	defaultCommissionMaxRate       = "0.2"
	defaultCommissionMaxChangeRate = "0.01"
	defaultMinSelfDelegation       = "1"
)

// NewTxCmd returns a root CLI command handler for all x/staking transaction commands.
func NewTxCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	stakingTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Staking transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	stakingTxCmd.AddCommand(
		NewCreateValidatorCmd(coinParser),
		stakingcli.NewEditValidatorCmd(),
		NewDelegateCmd(coinParser),
		NewRedelegateCmd(coinParser),
		NewUnbondCmd(coinParser),
	)

	return stakingTxCmd
}

func NewCreateValidatorCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-validator",
		Short: "create new validator initialized with a self-delegation to it",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.ReadTxCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			txf := tx.NewFactoryCLI(clientCtx, cmd.Flags()).WithTxConfig(clientCtx.TxConfig).WithAccountRetriever(clientCtx.AccountRetriever)

			txf, msg, err := NewBuildCreateValidatorMsg(clientCtx, txf, cmd.Flags(), coinParser)
			if err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxWithFactory(clientCtx, txf, msg)
		},
	}

	cmd.Flags().AddFlagSet(stakingcli.FlagSetPublicKey())
	cmd.Flags().AddFlagSet(stakingcli.FlagSetAmount())
	cmd.Flags().AddFlagSet(flagSetDescriptionCreate())
	cmd.Flags().AddFlagSet(stakingcli.FlagSetCommissionCreate())
	cmd.Flags().AddFlagSet(stakingcli.FlagSetMinSelfDelegation())

	cmd.Flags().String(stakingcli.FlagIP, "", fmt.Sprintf("The node's public IP. It takes effect only when used in combination with --%s", flags.FlagGenerateOnly))
	cmd.Flags().String(stakingcli.FlagNodeID, "", "The node's ID")
	flags.AddTxFlagsToCmd(cmd)

	_ = cmd.MarkFlagRequired(flags.FlagFrom)
	_ = cmd.MarkFlagRequired(stakingcli.FlagAmount)
	_ = cmd.MarkFlagRequired(stakingcli.FlagPubKey)
	_ = cmd.MarkFlagRequired(stakingcli.FlagMoniker)

	return cmd
}

func NewDelegateCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	bech32PrefixValAddr := sdk.GetConfig().GetBech32ValidatorAddrPrefix()

	cmd := &cobra.Command{
		Use:   "delegate [validator-addr] [amount]",
		Args:  cobra.ExactArgs(2),
		Short: "Delegate liquid tokens to a validator",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Delegate an amount of liquid coins to a validator from your wallet.
Example:
$ %s tx staking delegate %s1l2rsakp388kuv9k8qzq6lrm9taddae7fpx59wm 1000stake --from mykey
`,
				version.AppName, bech32PrefixValAddr,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.ReadTxCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			amount, err := coinParser.ParseCoin(args[1])
			if err != nil {
				return err
			}

			delAddr := clientCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			msg := types.NewMsgDelegate(delAddr, valAddr, amount)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewRedelegateCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	bech32PrefixValAddr := sdk.GetConfig().GetBech32ValidatorAddrPrefix()

	cmd := &cobra.Command{
		Use:   "redelegate [src-validator-addr] [dst-validator-addr] [amount]",
		Short: "Redelegate illiquid tokens from one validator to another",
		Args:  cobra.ExactArgs(3),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Redelegate an amount of illiquid staking tokens from one validator to another.
Example:
$ %s tx staking redelegate %s1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj %s1l2rsakp388kuv9k8qzq6lrm9taddae7fpx59wm 100stake --from mykey
`,
				version.AppName, bech32PrefixValAddr, bech32PrefixValAddr,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.ReadTxCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			delAddr := clientCtx.GetFromAddress()
			valSrcAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			valDstAddr, err := sdk.ValAddressFromBech32(args[1])
			if err != nil {
				return err
			}

			amount, err := coinParser.ParseCoin(args[2])
			if err != nil {
				return err
			}

			msg := types.NewMsgBeginRedelegate(delAddr, valSrcAddr, valDstAddr, amount)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewUnbondCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	bech32PrefixValAddr := sdk.GetConfig().GetBech32ValidatorAddrPrefix()

	cmd := &cobra.Command{
		Use:   "unbond [validator-addr] [amount]",
		Short: "Unbond shares from a validator",
		Args:  cobra.ExactArgs(2),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Unbond an amount of bonded shares from a validator.
Example:
$ %s tx staking unbond %s1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj 100stake --from mykey
`,
				version.AppName, bech32PrefixValAddr,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.ReadTxCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			delAddr := clientCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			amount, err := coinParser.ParseCoin(args[1])
			if err != nil {
				return err
			}

			msg := types.NewMsgUndelegate(delAddr, valAddr, amount)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewBuildCreateValidatorMsg(clientCtx client.Context, txf tx.Factory, fs *flag.FlagSet, coinParser chainsdk.CoinParser) (tx.Factory, sdk.Msg, error) {
	fAmount, _ := fs.GetString(stakingcli.FlagAmount)
	amount, err := coinParser.ParseCoin(fAmount)
	if err != nil {
		return txf, nil, err
	}

	valAddr := clientCtx.GetFromAddress()
	pkStr, _ := fs.GetString(stakingcli.FlagPubKey)

	pk, err := sdk.GetPubKeyFromBech32(sdk.Bech32PubKeyTypeConsPub, pkStr)
	if err != nil {
		return txf, nil, err
	}

	moniker, _ := fs.GetString(stakingcli.FlagMoniker)
	identity, _ := fs.GetString(stakingcli.FlagIdentity)
	website, _ := fs.GetString(stakingcli.FlagWebsite)
	security, _ := fs.GetString(stakingcli.FlagSecurityContact)
	details, _ := fs.GetString(stakingcli.FlagDetails)
	description := types.NewDescription(
		moniker,
		identity,
		website,
		security,
		details,
	)

	// get the initial validator commission parameters
	rateStr, _ := fs.GetString(stakingcli.FlagCommissionRate)
	maxRateStr, _ := fs.GetString(stakingcli.FlagCommissionMaxRate)
	maxChangeRateStr, _ := fs.GetString(stakingcli.FlagCommissionMaxChangeRate)

	commissionRates, err := buildCommissionRates(rateStr, maxRateStr, maxChangeRateStr)
	if err != nil {
		return txf, nil, err
	}

	// get the initial validator min self delegation
	msbStr, _ := fs.GetString(stakingcli.FlagMinSelfDelegation)

	minSelfDelegation, ok := sdk.NewIntFromString(msbStr)
	if !ok {
		return txf, nil, types.ErrMinSelfDelegationInvalid
	}

	msg := types.NewMsgCreateValidator(
		sdk.ValAddress(valAddr), pk, amount, description, commissionRates, minSelfDelegation,
	)
	if err := msg.ValidateBasic(); err != nil {
		return txf, nil, err
	}

	genOnly, _ := fs.GetBool(flags.FlagGenerateOnly)
	if genOnly {
		ip, _ := fs.GetString(stakingcli.FlagIP)
		nodeID, _ := fs.GetString(stakingcli.FlagNodeID)

		if nodeID != "" && ip != "" {
			txf = txf.WithMemo(fmt.Sprintf("%s@%s:26656", nodeID, ip))
		}
	}

	return txf, msg, nil
}

// Return the flagset, particular flags, and a description of defaults
// this is anticipated to be used with the gen-tx
func CreateValidatorMsgFlagSet(ipDefault string) (fs *flag.FlagSet, defaultsDesc string) {
	fsCreateValidator := flag.NewFlagSet("", flag.ContinueOnError)
	fsCreateValidator.String(stakingcli.FlagIP, ipDefault, "The node's public IP")
	fsCreateValidator.String(stakingcli.FlagNodeID, "", "The node's NodeID")
	fsCreateValidator.String(stakingcli.FlagMoniker, "", "The validator's (optional) moniker")
	fsCreateValidator.String(stakingcli.FlagWebsite, "", "The validator's (optional) website")
	fsCreateValidator.String(stakingcli.FlagSecurityContact, "", "The validator's (optional) security contact email")
	fsCreateValidator.String(stakingcli.FlagDetails, "", "The validator's (optional) details")
	fsCreateValidator.String(stakingcli.FlagIdentity, "", "The (optional) identity signature (ex. UPort or Keybase)")
	fsCreateValidator.AddFlagSet(stakingcli.FlagSetCommissionCreate())
	fsCreateValidator.AddFlagSet(stakingcli.FlagSetMinSelfDelegation())
	fsCreateValidator.AddFlagSet(stakingcli.FlagSetAmount())
	fsCreateValidator.AddFlagSet(stakingcli.FlagSetPublicKey())

	defaultsDesc = fmt.Sprintf(`
	delegation amount:           %s
	commission rate:             %s
	commission max rate:         %s
	commission max change rate:  %s
	minimum self delegation:     %s
`, defaultAmount, defaultCommissionRate,
		defaultCommissionMaxRate, defaultCommissionMaxChangeRate,
		defaultMinSelfDelegation)

	return fsCreateValidator, defaultsDesc
}

type TxCreateValidatorConfig struct {
	ChainID string
	NodeID  string
	Moniker string

	Amount string

	CommissionRate          string
	CommissionMaxRate       string
	CommissionMaxChangeRate string
	MinSelfDelegation       string

	PubKey string

	IP              string
	Website         string
	SecurityContact string
	Details         string
	Identity        string
}

func PrepareConfigForTxCreateValidator(flagSet *flag.FlagSet, moniker, nodeID, chainID string, valPubKey crypto.PubKey) (TxCreateValidatorConfig, error) {
	c := TxCreateValidatorConfig{}

	ip, err := flagSet.GetString(cli.FlagIP)
	if err != nil {
		return c, err
	}
	if ip == "" {
		_, _ = fmt.Fprintf(os.Stderr, "couldn't retrieve an external IP; "+
			"the tx's memo field will be unset")
	}
	c.IP = ip

	website, err := flagSet.GetString(cli.FlagWebsite)
	if err != nil {
		return c, err
	}
	c.Website = website

	securityContact, err := flagSet.GetString(cli.FlagSecurityContact)
	if err != nil {
		return c, err
	}
	c.SecurityContact = securityContact

	details, err := flagSet.GetString(cli.FlagDetails)
	if err != nil {
		return c, err
	}
	c.SecurityContact = details

	identity, err := flagSet.GetString(cli.FlagIdentity)
	if err != nil {
		return c, err
	}
	c.Identity = identity

	c.Amount, err = flagSet.GetString(cli.FlagAmount)
	if err != nil {
		return c, err
	}
	c.Amount, err = flagSet.GetString(cli.FlagAmount)
	if err != nil {
		return c, err
	}

	c.CommissionRate, err = flagSet.GetString(cli.FlagCommissionRate)
	if err != nil {
		return c, err
	}

	c.CommissionMaxRate, err = flagSet.GetString(cli.FlagCommissionMaxRate)
	if err != nil {
		return c, err
	}

	c.CommissionMaxChangeRate, err = flagSet.GetString(cli.FlagCommissionMaxChangeRate)
	if err != nil {
		return c, err
	}

	c.MinSelfDelegation, err = flagSet.GetString(cli.FlagMinSelfDelegation)
	if err != nil {
		return c, err
	}

	c.NodeID = nodeID
	c.PubKey = sdk.MustBech32ifyPubKey(sdk.Bech32PubKeyTypeConsPub, valPubKey)
	c.Website = website
	c.SecurityContact = securityContact
	c.Details = details
	c.Identity = identity
	c.ChainID = chainID
	c.Moniker = moniker

	if c.Amount == "" {
		c.Amount = defaultAmount
	}

	if c.CommissionRate == "" {
		c.CommissionRate = defaultCommissionRate
	}

	if c.CommissionMaxRate == "" {
		c.CommissionMaxRate = defaultCommissionMaxRate
	}

	if c.CommissionMaxChangeRate == "" {
		c.CommissionMaxChangeRate = defaultCommissionMaxChangeRate
	}

	if c.MinSelfDelegation == "" {
		c.MinSelfDelegation = defaultMinSelfDelegation
	}

	return c, nil
}

// BuildCreateValidatorMsg makes a new MsgCreateValidator.
func BuildCreateValidatorMsg(
	clientCtx client.Context,
	config TxCreateValidatorConfig,
	txBldr tx.Factory,
	generateOnly bool,
	coinParser chainsdk.CoinParser,
) (tx.Factory, sdk.Msg, error) {
	amounstStr := config.Amount
	amount, err := coinParser.ParseCoin(amounstStr)

	if err != nil {
		return txBldr, nil, err
	}

	valAddr := clientCtx.GetFromAddress()
	pkStr := config.PubKey

	pk, err := sdk.GetPubKeyFromBech32(sdk.Bech32PubKeyTypeConsPub, pkStr)
	if err != nil {
		return txBldr, nil, err
	}

	description := types.NewDescription(
		config.Moniker,
		config.Identity,
		config.Website,
		config.SecurityContact,
		config.Details,
	)

	// get the initial validator commission parameters
	rateStr := config.CommissionRate
	maxRateStr := config.CommissionMaxRate
	maxChangeRateStr := config.CommissionMaxChangeRate
	commissionRates, err := buildCommissionRates(rateStr, maxRateStr, maxChangeRateStr)

	if err != nil {
		return txBldr, nil, err
	}

	// get the initial validator min self delegation
	msbStr := config.MinSelfDelegation
	minSelfDelegation, ok := sdk.NewIntFromString(msbStr)

	if !ok {
		return txBldr, nil, types.ErrMinSelfDelegationInvalid
	}

	msg := types.NewMsgCreateValidator(
		sdk.ValAddress(valAddr), pk, amount, description, commissionRates, minSelfDelegation,
	)

	if generateOnly {
		ip := config.IP
		nodeID := config.NodeID

		if nodeID != "" && ip != "" {
			txBldr = txBldr.WithMemo(fmt.Sprintf("%s@%s:26656", nodeID, ip))
		}
	}

	return txBldr, msg, nil
}

func flagSetCommissionUpdate() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)

	fs.String(stakingcli.FlagCommissionRate, "", "The new commission rate percentage")

	return fs
}

func flagSetDescriptionCreate() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)

	fs.String(stakingcli.FlagMoniker, "", "The validator's name")
	fs.String(stakingcli.FlagIdentity, "", "The optional identity signature (ex. UPort or Keybase)")
	fs.String(stakingcli.FlagWebsite, "", "The validator's (optional) website")
	fs.String(stakingcli.FlagSecurityContact, "", "The validator's (optional) security contact email")
	fs.String(stakingcli.FlagDetails, "", "The validator's (optional) details")

	return fs
}
