package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/version"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"github.com/spf13/cobra"
)

var _ = strconv.Itoa(0)

func CmdSubmitTx() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-tx [connection-id] [msg_tx_json_file]",
		Short: "Submit a transaction on host chain on behalf of the interchain account",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Submit a transaction on host chain on behalf of the interchain account:
Example:
  $ %s tx %s submit-tx connection-1 tx.json --from mykey
  $ %s tx bank send <myaddress> <recipient> <amount> --generate-only > tx.json && %s tx %s submit-tx connection-1 tx.json --from mykey
			`, version.AppName, types.ModuleName, version.AppName, version.AppName, types.ModuleName),
		),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argConnectionID := args[0]
			argMsg := args[1]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			theTx, err := authclient.ReadTxFromFile(clientCtx, argMsg)
			if err != nil {
				return err
			}

			timeoutDuration, err := cmd.Flags().GetDuration(FlagTimeoutDuration)
			if err != nil {
				return err
			}

			msg := types.NewMsgSubmitTx(
				clientCtx.GetFromAddress().String(),
				argConnectionID,
				theTx.GetMsgs(),
				&timeoutDuration,
			)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().Duration(FlagTimeoutDuration, time.Minute*5, "Timeout duration for the transaction (default: 5 minutes)")

	return cmd
}
