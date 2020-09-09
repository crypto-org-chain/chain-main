package main

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	stakecmd "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	staketypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/crypto-com/chain-main/app"
)

// GetStakingTxCmd returns the transaction commands for staking
func GetStakingTxCmd(cdc *codec.Codec) *cobra.Command {
	stakingTxCmd := &cobra.Command{
		Use:                        staketypes.ModuleName,
		Short:                      "Staking transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	stakingTxCmd.AddCommand(flags.PostCommands(
		stakecmd.GetCmdCreateValidator(cdc),
		stakecmd.GetCmdEditValidator(cdc),
		CustomGetCmdDelegate(cdc),
		CustomGetCmdRedelegate(staketypes.StoreKey, cdc),
		CustomGetCmdUnbond(staketypes.StoreKey, cdc),
	)...)

	return stakingTxCmd
}

// CustomGetCmdDelegate implements the delegate command.
// converts CRO->carson
func CustomGetCmdDelegate(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "delegate [validator-addr] [amount]",
		Args:  cobra.ExactArgs(2),
		Short: "Delegate liquid tokens to a validator",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Delegate an amount of liquid coins to a validator from your wallet.

Example:
$ %s tx staking delegate crocncl152gc4l936x4xpcf7nasat4ldfmpmg29s4mt45p 1000cro --from mykey
`,
				version.ClientName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(auth.DefaultTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			amount, parseerr := sdk.ParseCoin(args[1])
			if parseerr != nil {
				return parseerr
			}

			camount, converterr := sdk.ConvertCoin(amount, app.BaseCoinUnit)
			if converterr != nil {
				return converterr
			}

			delAddr := cliCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			msg := staketypes.NewMsgDelegate(delAddr, valAddr, camount)
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}

// CustomGetCmdRedelegate the begin redelegation command.
// converts CRO->carson
func CustomGetCmdRedelegate(storeName string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "redelegate [src-validator-addr] [dst-validator-addr] [amount]",
		Short: "Redelegate illiquid tokens from one validator to another",
		Args:  cobra.ExactArgs(3),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Redelegate an amount of illiquid staking tokens from one validator to another.

Example:
$ %s tx staking redelegate crocncl152gc4l936x4xpcf7nasat4ldfmpmg29s4mt45p 
crocncl1v3mdsef3l6jqjnqn8yglsem43j487a35va27as 100cro --from mykey
`,
				version.ClientName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(auth.DefaultTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			delAddr := cliCtx.GetFromAddress()
			valSrcAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			valDstAddr, errdst := sdk.ValAddressFromBech32(args[1])
			if errdst != nil {
				return errdst
			}

			amount, parseerr := sdk.ParseCoin(args[1])
			if parseerr != nil {
				return parseerr
			}
			camount, converterr := sdk.ConvertCoin(amount, app.BaseCoinUnit)
			if converterr != nil {
				return converterr
			}

			msg := staketypes.NewMsgBeginRedelegate(delAddr, valSrcAddr, valDstAddr, camount)
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}

// CustomGetCmdUnbond implements the unbond validator command.
// converts CRO->carson
func CustomGetCmdUnbond(storeName string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "unbond [validator-addr] [amount]",
		Short: "Unbond shares from a validator",
		Args:  cobra.ExactArgs(2),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Unbond an amount of bonded shares from a validator.

Example:
$ %s tx staking unbond crocncl152gc4l936x4xpcf7nasat4ldfmpmg29s4mt45p 100cro --from mykey
`,
				version.ClientName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(auth.DefaultTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			delAddr := cliCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			amount, parseerr := sdk.ParseCoin(args[1])
			if parseerr != nil {
				return parseerr
			}
			camount, converterr := sdk.ConvertCoin(amount, app.BaseCoinUnit)
			if converterr != nil {
				return converterr
			}

			msg := staketypes.NewMsgUndelegate(delAddr, valAddr, camount)
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}
