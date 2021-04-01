package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/cosmos/cosmos-sdk/version"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/crypto-org-chain/chain-main/v1/config"
	"github.com/crypto-org-chain/chain-main/v1/x/chainmain/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group chainmain queries under a subcommand
	chainmainQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	flags.AddQueryFlagsToCmd(chainmainQueryCmd)

	chainmainQueryCmd.AddCommand(GetBalancesCmd())

	return chainmainQueryCmd
}

func QueryAllTxCmd() *cobra.Command {
	// returns a command to search through transactions by address.
	cmd := &cobra.Command{
		Use:   "txs-all [address]",
		Short: "Query for all  paginated transactions by address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, queryErr := client.GetClientQueryContext(cmd)
			if queryErr != nil {
				return queryErr
			}
			addrStr := args[0]
			_, addressErr := sdk.AccAddressFromBech32(addrStr)
			if addressErr != nil {
				return addressErr
			}

			var tmEvents []string
			events := [...]string{"message.sender", "transfer.recipient", "withdraw_rewards.validator", "ibc_transfer.receiver", "ibc_transfer.sender"}
			for _, event := range events {
				eventPair := fmt.Sprintf("%s='%s'", event, addrStr)
				tmEvents = append(tmEvents, eventPair)
			}

			page, pageErr := cmd.Flags().GetInt(flags.FlagPage)
			if pageErr != nil {
				return pageErr
			}
			limit, limitErr := cmd.Flags().GetInt(flags.FlagLimit)
			if limitErr != nil {
				return limitErr
			}
			txs, err := authclient.QueryTxsByEvents(clientCtx, tmEvents, page, limit, "")
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(txs)
		},
	}
	cmd.Flags().StringP(flags.FlagNode, "n", "tcp://localhost:26657", "Node to connect to")
	cmd.Flags().String(flags.FlagKeyringBackend, flags.DefaultKeyringBackend, "Select keyring's backend (os|file|kwallet|pass|test)")
	cmd.Flags().Int(flags.FlagPage, rest.DefaultPage, "Query a specific page of paginated results")
	cmd.Flags().Int(flags.FlagLimit, rest.DefaultLimit, "Query number of transactions results per page returned")
	return cmd
}

func GetBalancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balances [address]",
		Short: "Query for account balances by address",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query the total balance of an account.
Example:
  $ %s query %s balances [address]
`,
				version.AppName, types.ModuleName,
			),
		),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, queryErr := client.GetClientQueryContext(cmd)
			clientCtx = clientCtx.WithNodeURI("http://localhost:26657")
			if queryErr != nil {
				return queryErr
			}

			queryClient := banktypes.NewQueryClient(clientCtx)

			addr, addressErr := sdk.AccAddressFromBech32(args[0])
			if addressErr != nil {
				return addressErr
			}

			pageReq, clientErr := client.ReadPageRequest(cmd.Flags())
			if clientErr != nil {
				return clientErr
			}

			params := banktypes.NewQueryAllBalancesRequest(addr, pageReq)
			res, allBalancesErr := queryClient.AllBalances(context.Background(), params)
			if allBalancesErr != nil {
				return allBalancesErr
			}

			baseUnit, err := sdk.GetBaseDenom()
			if err != nil {
				return err
			}
			baseAmount := res.Balances.AmountOf(baseUnit)
			humanCoin, err := sdk.ConvertDecCoin(sdk.NewDecCoin(baseUnit, baseAmount), config.HumanCoinUnit)
			if err != nil {
				return err
			}
			fmt.Printf(
				"%s CRO (%s baseCRO)\n",
				humanCoin.String(),
				baseAmount.String(),
			)
			return nil
		},
	}

	flags.AddPaginationFlagsToCmd(cmd, "all balances")

	return cmd
}
