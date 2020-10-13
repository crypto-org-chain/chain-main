package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/crypto-com/chain-main/config"
	"github.com/crypto-com/chain-main/x/chainmain/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(coinParser types.CoinParser) *cobra.Command {
	// Group chainmain queries under a subcommand
	chainmainQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	flags.AddQueryFlagsToCmd(chainmainQueryCmd)

	chainmainQueryCmd.AddCommand(GetBalancesCmd(coinParser))

	return chainmainQueryCmd
}

func GetBalancesCmd(coinParser types.CoinParser) *cobra.Command {
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
			clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, queryErr := client.ReadQueryCommandFlags(clientCtx, cmd.Flags())
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

			baseUnit := coinParser.GetBaseUnit()
			baseAmount := res.Balances.AmountOf(baseUnit)
			fmt.Printf(
				"%s CRO (%s baseCRO)\n",
				coinParser.MustSprintBaseCoin(baseAmount, config.HumanCoinUnit),
				baseAmount.String(),
			)
			return nil
		},
	}

	flags.AddPaginationFlagsToCmd(cmd, "all balances")

	return cmd
}
