package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/chain-main/v2/x/subscription/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group subscription queries under a subcommand
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the subscription module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		GetCmdQueryPlan(),
		GetCmdQueryPlans(),
		GetCmdQuerySubscription(),
		GetCmdQuerySubscriptions(),
	)

	return queryCmd
}

// GetCmdQueryPlan implements the query plan command.
func GetCmdQueryPlan() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan [plan-id]",
		Args:  cobra.ExactArgs(1),
		Short: "Query details of a single plan",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			// validate that the plan id is a uint
			planID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("plan-id %s not a valid uint, please input a valid plan-id", args[0])
			}

			// Query the plan
			res, err := queryClient.Plan(
				cmd.Context(),
				&types.QueryPlanRequest{PlanId: planID},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Plan)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryPlans implements a query plans command. Command to Get a
// Plan Information.
func GetCmdQueryPlans() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plans",
		Short: "Query plans with optional filters",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, err := cmd.Flags().GetString(flagOwner)
			if err != nil {
				return err
			}

			if len(owner) != 0 {
				_, err = sdk.AccAddressFromBech32(owner)
				if err != nil {
					return err
				}
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.Plans(
				cmd.Context(),
				&types.QueryPlansRequest{
					Owner:      owner,
					Pagination: pageReq,
				},
			)
			if err != nil {
				return err
			}

			if len(res.GetPlans()) == 0 {
				return fmt.Errorf("no plans found")
			}

			return clientCtx.PrintProto(res)
		},
	}

	cmd.Flags().String(flagOwner, "", "(optional) filter by plans deposited on by depositor")
	flags.AddPaginationFlagsToCmd(cmd, "plans")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQuerySubscription implements the query subscription command.
func GetCmdQuerySubscription() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscription [subscription-id]",
		Args:  cobra.ExactArgs(1),
		Short: "Query details of a single subscription",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			// validate that the subscription id is a uint
			subscriptionID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("subscription-id %s not a valid uint, please input a valid subscription-id", args[0])
			}

			// Query the subscription
			res, err := queryClient.Subscription(
				cmd.Context(),
				&types.QuerySubscriptionRequest{SubscriptionId: subscriptionID},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Subscription)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQuerySubscriptions implements a query subscriptions command. Command to Get a
// Subscription Information.
func GetCmdQuerySubscriptions() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Query subscriptions with optional filters",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			subscriber, err := cmd.Flags().GetString(flagSubscriber)
			if err != nil {
				return err
			}

			if len(subscriber) != 0 {
				_, err = sdk.AccAddressFromBech32(subscriber)
				if err != nil {
					return err
				}
			}

			planID, err := cmd.Flags().GetInt64(flagPlanID)
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			res, err := queryClient.Subscriptions(
				cmd.Context(),
				&types.QuerySubscriptionsRequest{
					PlanId:     planID,
					Subscriber: subscriber,
					Pagination: pageReq,
				},
			)
			if err != nil {
				return err
			}

			if len(res.GetSubscriptions()) == 0 {
				return fmt.Errorf("no subscriptions found")
			}

			return clientCtx.PrintProto(res)
		},
	}

	cmd.Flags().String(flagOwner, "", "(optional) filter by subscriptions deposited on by depositor")
	cmd.Flags().Int64(flagPlanID, -1, "(optional) filter by subscriptions deposited on by depositor")
	flags.AddPaginationFlagsToCmd(cmd, "subscriptions")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
