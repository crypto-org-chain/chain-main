package cli

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v4/x/maxsupply/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/version"
)

// GetQueryCmd returns the parent command for all x/maxsupply CLI query commands.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the maxsupply module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdQueryMaxSupply(),
		GetCmdQueryBurnedAddresses(),
	)

	return cmd
}

// GetCmdQueryMaxSupply implements the max supply query command.
func GetCmdQueryMaxSupply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "max-supply",
		Args:  cobra.NoArgs,
		Short: "Query the maximum supply",
		Long: fmt.Sprintf(`Query the maximum supply value.

Example:
$ %s query maxsupply max-supply
`,
			version.AppName,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.MaxSupply(context.Background(), &types.QueryMaxSupplyRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryBurnedAddresses implements the burned addresses query command.
func GetCmdQueryBurnedAddresses() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burned-addresses",
		Args:  cobra.NoArgs,
		Short: "Query the list of burned addresses",
		Long: fmt.Sprintf(`Query the list of addresses that hold burned tokens.

Example:
$ %s query maxsupply burned-addresses
`,
			version.AppName,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.BurnedAddresses(context.Background(), &types.QueryBurnedAddressesRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
