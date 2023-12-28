package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/version"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	ibctypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	"github.com/crypto-org-chain/chain-main/v4/config"
	"github.com/crypto-org-chain/chain-main/v4/x/chainmain/types"
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
			events := []string{
				// sender/receiver in send.go
				fmt.Sprintf("%s.%s='%s'", sdk.EventTypeMessage, sdk.AttributeKeySender, addrStr),
				fmt.Sprintf("%s.%s='%s'", banktypes.EventTypeTransfer, sdk.AttributeKeySender, addrStr),
				fmt.Sprintf("%s.%s='%s'", banktypes.EventTypeTransfer, banktypes.AttributeKeyRecipient, addrStr),
				// ibc transfer
				fmt.Sprintf("%s.%s='%s'", ibctypes.EventTypeTransfer, sdk.AttributeKeySender, addrStr),
				fmt.Sprintf("%s.%s='%s'", ibctypes.EventTypeTransfer, ibctypes.AttributeKeyReceiver, addrStr),
				// in SetWithdrawAddress
				fmt.Sprintf("%s.%s='%s'", disttypes.EventTypeSetWithdrawAddress, disttypes.AttributeKeyWithdrawAddress, addrStr),
				// in WithdrawDelegationRewards
				fmt.Sprintf("%s.%s='%s'", disttypes.EventTypeWithdrawRewards, disttypes.AttributeKeyValidator, addrStr),
				// in AllocateTokens
				fmt.Sprintf("%s.%s='%s'", disttypes.EventTypeProposerReward, disttypes.AttributeKeyValidator, addrStr),
				// in AllocateTokensToValidator
				fmt.Sprintf("%s.%s='%s'", disttypes.EventTypeCommission, disttypes.AttributeKeyValidator, addrStr),
				fmt.Sprintf("%s.%s='%s'", disttypes.EventTypeRewards, disttypes.AttributeKeyValidator, addrStr),
			}

			page, pageErr := cmd.Flags().GetInt(flags.FlagPage)
			if pageErr != nil {
				return pageErr
			}
			limit, limitErr := cmd.Flags().GetInt(flags.FlagLimit)
			if limitErr != nil {
				return limitErr
			}
			txsResult := sdk.SearchTxsResult{}
			txsMap := map[string]*sdk.TxResponse{}
			for _, event := range events {
				txs, err := authtx.QueryTxsByEvents(clientCtx, []string{event}, page, limit, "")
				if err != nil {
					return nil
				}
				for _, tx := range txs.Txs {
					txsMap[tx.TxHash] = tx
				}
				txsResult.PageTotal = txs.PageTotal
				txsResult.PageNumber = txs.PageNumber
				txsResult.Limit = txs.Limit
			}
			for _, tx := range txsMap {
				txsResult.Txs = append(txsResult.Txs, tx)
			}
			txsResult.TotalCount = uint64(len(txsResult.Txs))
			txsResult.Count = txsResult.TotalCount
			return clientCtx.PrintProto(&txsResult)
		},
	}
	cmd.Flags().StringP(flags.FlagNode, "n", "tcp://localhost:26657", "Node to connect to")
	cmd.Flags().String(flags.FlagKeyringBackend, flags.DefaultKeyringBackend, "Select keyring's backend (os|file|kwallet|pass|test)")
	cmd.Flags().Int(flags.FlagPage, query.DefaultPage, "Query a specific page of paginated results")
	cmd.Flags().Int(flags.FlagLimit, query.DefaultLimit, "Query number of transactions results per page returned")
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
