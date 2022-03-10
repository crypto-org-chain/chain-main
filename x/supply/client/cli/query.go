package cli

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/crypto-org-chain/chain-main/v4/x/supply/types"
	"github.com/spf13/cobra"
)

// GetQueryCmd returns the parent command for all x/supply CLI query commands. The
// provided clientCtx should have, at a minimum, a verifier, Tendermint RPC client,
// and marshaler set.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the supply module [Deprecated: do not use]",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdQueryTotalSupply(),
		GetCmdQueryLiquidSupply(),
	)

	return cmd
}

// GetCmdQueryTotalSupply returns command for total supply
func GetCmdQueryTotalSupply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "total",
		Short: "Query the total supply of coins of the chain [Deprecated: do not use]",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query total supply of coins that are held by accounts in the chain. [Deprecated: do not use]
Example:
  $ %s query %s total
`,
				version.AppName, types.ModuleName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// nolint: staticcheck
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.TotalSupply(cmd.Context(), types.NewSupplyRequest())
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryLiquidSupply returns command for total supply
func GetCmdQueryLiquidSupply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "liquid",
		Short: "Query the liquid supply of coins of the chain [Deprecated: do not use]",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query liquid supply of coins that are held by accounts in the chain. [Deprecated: do not use]
Example:
  $ %s query %s liquid
`,
				version.AppName, types.ModuleName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// nolint: staticcheck
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.LiquidSupply(cmd.Context(), types.NewSupplyRequest())
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
