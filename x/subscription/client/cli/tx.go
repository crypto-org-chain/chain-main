package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/chain-main/v2/x/subscription/types"
)

// subscription flags
const (
	FlagTitle          = "title"
	FlagDescription    = "description"
	FlagPrice          = "price"
	flagDurationSecs   = "duration-secs"
	flagCronSpec       = "cron-spec"
	flagPlanID         = "plan-id"
	flagSubscriptionID = "subscription-id"
	flagOwner          = "owner"
	flagSubscriber     = "subscriber"
	flagTzoffset       = "tzoffset"
)

// nolint: dupl
func NewTxCmd() *cobra.Command {
	subscriptionTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Subscription transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	subscriptionTxCmd.AddCommand(
		NewCmdCreatePlan(),
		NewCmdStopPlan(),
		NewCmdCreateSubscription(),
		NewCmdStopSubscription(),
		NewCmdStopUserSubscription(),
	)

	return subscriptionTxCmd
}

// NewCmdCreatePlan implements submitting a create plan transaction command.
func NewCmdCreatePlan() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-plan",
		Short: "Create a subscription plan",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			// Get owner address
			owner := clientCtx.GetFromAddress()

			flags := cmd.Flags()

			title, err := flags.GetString(FlagTitle)
			if err != nil {
				return err
			}
			description, err := flags.GetString(FlagDescription)
			if err != nil {
				return err
			}
			strPrice, err := flags.GetString(FlagPrice)
			if err != nil {
				return err
			}
			price, err := sdk.ParseCoinsNormalized(strPrice)
			if err != nil {
				return err
			}
			durationSecs, err := flags.GetUint32(flagDurationSecs)
			if err != nil {
				return err
			}
			strSpec, err := flags.GetString(flagCronSpec)
			if err != nil {
				return err
			}
			spec, err := types.ParseCronSpec(strSpec)
			if err != nil {
				return err
			}
			tzoffset, err := flags.GetInt32(flagTzoffset)
			if err != nil {
				return err
			}
			msg := types.MsgCreatePlan{
				Owner:        owner.String(),
				Title:        title,
				Description:  description,
				Price:        price,
				DurationSecs: durationSecs,
				CronSpec:     spec,
				Tzoffset:     tzoffset,
			}

			if err = msg.ValidateBasic(); err != nil {
				return fmt.Errorf("message validation failed: %w", err)
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	cmd.Flags().String(FlagTitle, "", "The plan title")
	cmd.Flags().String(FlagDescription, "", "The plan description")
	cmd.Flags().String(FlagPrice, "", "The plan price")
	cmd.Flags().Uint32(flagDurationSecs, 365*24*3600, "The plan subscription duration in seconds")
	cmd.Flags().String(flagCronSpec, "0 0 1 * *", "The plan cron spec")
	cmd.Flags().Int32(flagTzoffset, 0, "The timezone offset in seconds")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// nolint: dupl
func NewCmdStopPlan() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop-plan",
		Short: "Stop a subscription plan",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			// Get owner address
			owner := clientCtx.GetFromAddress()

			flags := cmd.Flags()
			planID, err := flags.GetUint64(flagPlanID)
			if err != nil {
				return err
			}

			msg := types.MsgStopPlan{
				Owner:  owner.String(),
				PlanId: planID,
			}

			if err = msg.ValidateBasic(); err != nil {
				return fmt.Errorf("message validation failed: %w", err)
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	cmd.Flags().Uint64(flagPlanID, 0, "The plan id")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// nolint: dupl
func NewCmdCreateSubscription() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscribe",
		Short: "Subscribe to a plan",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			// Get subscriber address
			subscriber := clientCtx.GetFromAddress()

			flags := cmd.Flags()
			planID, err := flags.GetUint64(flagPlanID)
			if err != nil {
				return err
			}

			msg := types.MsgCreateSubscription{
				Subscriber: subscriber.String(),
				PlanId:     planID,
			}

			if err = msg.ValidateBasic(); err != nil {
				return fmt.Errorf("message validation failed: %w", err)
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	cmd.Flags().Uint64(flagPlanID, 0, "The plan id")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// nolint: dupl
func NewCmdStopSubscription() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unsubscribe",
		Short: "Unsubscribe to a plan",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			// Get subscriber address
			subscriber := clientCtx.GetFromAddress()

			flags := cmd.Flags()
			subscriptionID, err := flags.GetUint64(flagSubscriptionID)
			if err != nil {
				return err
			}

			msg := types.MsgStopSubscription{
				Subscriber:     subscriber.String(),
				SubscriptionId: subscriptionID,
			}

			if err = msg.ValidateBasic(); err != nil {
				return fmt.Errorf("message validation failed: %w", err)
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	cmd.Flags().Uint64(flagSubscriptionID, 0, "The subscription id")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// nolint: dupl
func NewCmdStopUserSubscription() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unsubscribe-user",
		Short: "Plan owner unsubscribe a user",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			// Get owner address
			owner := clientCtx.GetFromAddress()

			flags := cmd.Flags()
			subscriptionID, err := flags.GetUint64(flagSubscriptionID)
			if err != nil {
				return err
			}

			msg := types.MsgStopUserSubscription{
				PlanOwner:      owner.String(),
				SubscriptionId: subscriptionID,
			}

			if err = msg.ValidateBasic(); err != nil {
				return fmt.Errorf("message validation failed: %w", err)
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	cmd.Flags().Uint64(flagSubscriptionID, 0, "The subscription id")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
