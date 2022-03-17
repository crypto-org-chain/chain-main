package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"github.com/spf13/cobra"
)

var _ = strconv.Itoa(0)

func CmdInterchainAccountAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interchain-account-address [connection-id] [owner]",
		Short: "Gets interchain account address on host chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqConnectionID := args[0]
			reqOwner := args[1]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			params := &types.QueryInterchainAccountAddressRequest{
				ConnectionId: reqConnectionID,
				Owner:        reqOwner,
			}

			res, err := queryClient.InterchainAccountAddress(cmd.Context(), params)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
