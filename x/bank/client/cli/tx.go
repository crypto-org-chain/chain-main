package cli

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/spf13/cobra"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
)

// NewTxCmd returns a root CLI command handler for all x/bank transaction commands.
func NewTxCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Bank transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(NewSendTxCmd(coinParser))
	txCmd.AddCommand(NewMultiSend1ToManyTxCmd(coinParser))

	return txCmd
}

// NewSendTxCmd returns a CLI command handler for creating a MsgSend transaction.
func NewSendTxCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	bech32PrefixAddr := sdk.GetConfig().GetBech32AccountAddrPrefix()

	cmd := &cobra.Command{
		Use:   "send [from_key_or_address] [to_address] [amount]",
		Short: `Send funds from one account to another.`,
		Long: strings.TrimSpace(
			//nolint:lll
			fmt.Sprintf(`Send funds from one account to another. Note, the'--from' flag is ignored as it is implied from [from_key_or_address].
Example:
$ %s tx bank send %s158ecvsn6wccdf3knmser5vk53nxk5sr %s1d8kyuler5axgh6pndshqx6sfttes3jq5jnswdt 1000cro
		`,
				version.AppName, bech32PrefixAddr, bech32PrefixAddr,
			),
		),
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			//nolint:errcheck
			cmd.Flags().Set(flags.FlagFrom, args[0])

			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.ReadTxCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			toAddr, err := sdk.AccAddressFromBech32(args[1])
			if err != nil {
				return err
			}

			coins, err := coinParser.ParseCoins(args[2])
			if err != nil {
				return err
			}

			msg := types.NewMsgSend(clientCtx.GetFromAddress(), toAddr, coins)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewMultiSend1ToManyTxCmd returns a CLI command handler for creating a 1-to-many MsgMultiSend transaction.
func NewMultiSend1ToManyTxCmd(coinParser chainsdk.CoinParser) *cobra.Command {
	bech32PrefixAddr := sdk.GetConfig().GetBech32AccountAddrPrefix()

	cmd := &cobra.Command{
		Use:   "multisend-1tomany [from_key_or_address] [to_address] [to_amount] [to_address2] [to_amount2] ...",
		Short: `Send funds in batch from one account to multiple outputs.`,
		Long: strings.TrimSpace(
			//nolint:lll
			fmt.Sprintf(`Send funds from one account to multiple accounts. Note, the'--from' flag is ignored as it is implied from [from_key_or_address].
Example:
$ %s tx bank multisend-1tomany %s158ecvsn6wccdf3knmser5vk53nxk5sr %s1d8kyuler5axgh6pndshqx6sfttes3jq5jnswdt 1000cro %s1kd94yh0xauy29hzw4dh84hd3xmunez46evzv76 2000cro
		`,
				version.AppName, bech32PrefixAddr, bech32PrefixAddr, bech32PrefixAddr,
			),
		),
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromAddress := args[0]
			//nolint:errcheck
			cmd.Flags().Set(flags.FlagFrom, fromAddress)

			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.ReadTxCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			fromCoin := sdk.NewCoin(coinParser.GetBaseUnit(), sdk.ZeroInt())
			outputs := make([]types.Output, 0)

			if (len(args)-1)%2 != 0 {
				return fmt.Errorf("expected recipient address and amount come in pairs, found odd number of arguments")
			}

			for i := 1; i < len(args); i += 2 {
				toAddr, err := sdk.AccAddressFromBech32(args[i])
				if err != nil {
					return err
				}

				coins, err := coinParser.ParseCoins(args[i+1])
				if err != nil {
					return err
				}

				outputs = append(outputs, types.Output{
					Address: toAddr.String(),
					Coins:   coins,
				})
				fromCoin = fromCoin.Add(sdk.NewCoin(coinParser.GetBaseUnit(), coins.AmountOf(coinParser.GetBaseUnit())))
			}
			inputs := []types.Input{
				{
					Address: fromAddress,
					Coins:   sdk.NewCoins(fromCoin),
				},
			}

			msg := types.NewMsgMultiSend(inputs, outputs)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
