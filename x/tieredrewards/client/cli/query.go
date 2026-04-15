package cli

import (
	"context"

	"github.com/cosmos/gogoproto/proto"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type queryRunFunc func(ctx context.Context, clientCtx client.Context, queryClient types.QueryClient, args []string) (proto.Message, error)

// GetQueryCmd returns the query commands for the tieredrewards module.
func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the tieredrewards module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		GetCmdQueryParams(),
		GetCmdQueryTierPosition(),
		GetCmdQueryTierPositionsByOwner(),
		GetCmdQueryAllTierPositions(),
		GetCmdQueryTiers(),
		GetCmdQueryRewardsPoolBalance(),
		GetCmdQueryEstimatePositionRewards(),
		GetCmdQueryVotingPowerByOwner(),
		GetCmdQueryTotalDelegatedVotingPower(),
	)

	return queryCmd
}

func newQueryCmd(
	use string,
	args cobra.PositionalArgs,
	short string,
	run queryRunFunc,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Args:  args,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			res, err := run(cmd.Context(), clientCtx, types.NewQueryClient(clientCtx), args)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func newPaginatedQueryCmd(
	use string,
	short string,
	run func(context.Context, client.Context, *cobra.Command, types.QueryClient) (proto.Message, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Args:  cobra.NoArgs,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			res, err := run(cmd.Context(), clientCtx, cmd, types.NewQueryClient(clientCtx))
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func GetCmdQueryParams() *cobra.Command {
	return newQueryCmd(
		"params",
		cobra.NoArgs,
		"Query the current tieredrewards parameters",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, _ []string) (proto.Message, error) {
			return queryClient.Params(ctx, &types.QueryParamsRequest{})
		},
	)
}

func GetCmdQueryTierPosition() *cobra.Command {
	return newQueryCmd(
		"position [position-id]",
		cobra.ExactArgs(1),
		"Query a single tier position by ID",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, args []string) (proto.Message, error) {
			positionID, err := parseUint64Arg("position-id", args[0])
			if err != nil {
				return nil, err
			}

			return queryClient.TierPosition(ctx, &types.QueryTierPositionRequest{
				PositionId: positionID,
			})
		},
	)
}

func GetCmdQueryTierPositionsByOwner() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "positions-by-owner [owner]",
		Args:  cobra.ExactArgs(1),
		Short: "Query all tier positions for an owner address",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			if _, err := sdk.AccAddressFromBech32(args[0]); err != nil {
				return err
			}

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.TierPositionsByOwner(cmd.Context(), &types.QueryTierPositionsByOwnerRequest{
				Owner:      args[0],
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "positions-by-owner")
	return cmd
}

func GetCmdQueryAllTierPositions() *cobra.Command {
	cmd := newPaginatedQueryCmd(
		"positions",
		"Query all tier positions (paginated)",
		func(ctx context.Context, _ client.Context, cmd *cobra.Command, queryClient types.QueryClient) (proto.Message, error) {
			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return nil, err
			}

			return queryClient.AllTierPositions(ctx, &types.QueryAllTierPositionsRequest{
				Pagination: pageReq,
			})
		},
	)
	flags.AddPaginationFlagsToCmd(cmd, "positions")
	return cmd
}

func GetCmdQueryTiers() *cobra.Command {
	return newQueryCmd(
		"tiers",
		cobra.NoArgs,
		"Query all tier definitions",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, _ []string) (proto.Message, error) {
			return queryClient.Tiers(ctx, &types.QueryTiersRequest{})
		},
	)
}

func GetCmdQueryRewardsPoolBalance() *cobra.Command {
	return newQueryCmd(
		"rewards-pool-balance",
		cobra.NoArgs,
		"Query the rewards pool balance",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, _ []string) (proto.Message, error) {
			return queryClient.RewardsPoolBalance(ctx, &types.QueryRewardsPoolBalanceRequest{})
		},
	)
}

func GetCmdQueryEstimatePositionRewards() *cobra.Command {
	return newQueryCmd(
		"estimate-position-rewards [position-id]",
		cobra.ExactArgs(1),
		"Estimate pending base and bonus rewards for a position",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, args []string) (proto.Message, error) {
			positionID, err := parseUint64Arg("position-id", args[0])
			if err != nil {
				return nil, err
			}

			return queryClient.EstimatePositionRewards(ctx, &types.QueryEstimatePositionRewardsRequest{
				PositionId: positionID,
			})
		},
	)
}

func GetCmdQueryVotingPowerByOwner() *cobra.Command {
	return newQueryCmd(
		"voting-power [owner]",
		cobra.ExactArgs(1),
		"Query governance voting power from delegated tier positions for an owner address",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, args []string) (proto.Message, error) {
			if _, err := sdk.AccAddressFromBech32(args[0]); err != nil {
				return nil, err
			}

			return queryClient.VotingPowerByOwner(ctx, &types.QueryVotingPowerByOwnerRequest{
				Owner: args[0],
			})
		},
	)
}

func GetCmdQueryTotalDelegatedVotingPower() *cobra.Command {
	return newQueryCmd(
		"total-delegated-voting-power",
		cobra.NoArgs,
		"Query total governance voting power from delegated tier positions",
		func(ctx context.Context, _ client.Context, queryClient types.QueryClient, _ []string) (proto.Message, error) {
			return queryClient.TotalDelegatedVotingPower(ctx, &types.QueryTotalDelegatedVotingPowerRequest{})
		},
	)
}
