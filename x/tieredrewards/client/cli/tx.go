package cli

import (
	"fmt"
	"strconv"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetTxCmd returns the transaction commands for the tieredrewards module.
// Only commands that use cosmos.base.v1beta1.Coin fields are defined here
// (autocli panics on dynamicpb proto.Merge for Coin messages without pulsar types).
// All other tx commands are handled by autocli.
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewLockTierCmd(),
		NewCommitDelegationToTierCmd(),
		NewAddToTierPositionCmd(),
		NewFundTierPoolCmd(),
	)

	return txCmd
}

// NewLockTierCmd creates a CLI command for MsgLockTier.
func NewLockTierCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock-tier",
		Short: "Lock tokens into a tier with optional delegate and exit options",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			tierID, err := cmd.Flags().GetUint32("tier-id")
			if err != nil {
				return err
			}

			amountStr, err := cmd.Flags().GetString("amount")
			if err != nil {
				return err
			}
			amount, err := sdk.ParseCoinNormalized(amountStr)
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}

			validator, _ := cmd.Flags().GetString("validator")
			triggerExit, _ := cmd.Flags().GetBool("trigger-exit-immediately")

			msg := &types.MsgLockTier{
				Owner:                  clientCtx.GetFromAddress().String(),
				TierId:                 tierID,
				Amount:                 amount,
				Validator:              validator,
				TriggerExitImmediately: triggerExit,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint32("tier-id", 0, "Tier ID to lock into")
	cmd.Flags().String("amount", "", "Amount to lock (e.g. 1000basecro)")
	cmd.Flags().String("validator", "", "Validator to delegate to at lock time (optional)")
	cmd.Flags().Bool("trigger-exit-immediately", false, "Start exit commitment from creation (optional)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewCommitDelegationToTierCmd creates a CLI command for MsgCommitDelegationToTier.
func NewCommitDelegationToTierCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit-delegation-to-tier",
		Short: "Commit an existing delegation to a tier without undelegating",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			tierID, err := cmd.Flags().GetUint32("tier-id")
			if err != nil {
				return err
			}

			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}

			amountStr, err := cmd.Flags().GetString("amount")
			if err != nil {
				return err
			}
			amount, err := sdk.ParseCoinNormalized(amountStr)
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}

			msg := &types.MsgCommitDelegationToTier{
				Owner:     clientCtx.GetFromAddress().String(),
				TierId:    tierID,
				Validator: validator,
				Amount:    amount,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint32("tier-id", 0, "Tier ID to commit into")
	cmd.Flags().String("validator", "", "Validator address of the existing delegation")
	cmd.Flags().String("amount", "", "Amount to commit (e.g. 5000basecro)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewAddToTierPositionCmd creates a CLI command for MsgAddToTierPosition.
func NewAddToTierPositionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-to-tier-position",
		Short: "Add tokens to an existing tier position",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			posIDStr, err := cmd.Flags().GetString("position-id")
			if err != nil {
				return err
			}
			posID, err := strconv.ParseUint(posIDStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid position-id: %w", err)
			}

			amountStr, err := cmd.Flags().GetString("amount")
			if err != nil {
				return err
			}
			amount, err := sdk.ParseCoinNormalized(amountStr)
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}

			msg := &types.MsgAddToTierPosition{
				Owner:      clientCtx.GetFromAddress().String(),
				PositionId: posID,
				Amount:     amount,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("position-id", "", "Position ID to add tokens to")
	cmd.Flags().String("amount", "", "Amount to add (e.g. 2000basecro)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewFundTierPoolCmd creates a CLI command for MsgFundTierPool.
func NewFundTierPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fund-tier-pool",
		Short: "Fund the tier rewards pool",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			amountStr, err := cmd.Flags().GetString("amount")
			if err != nil {
				return err
			}
			coins, err := sdk.ParseCoinsNormalized(amountStr)
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}

			msg := &types.MsgFundTierPool{
				Sender: clientCtx.GetFromAddress().String(),
				Amount: coins,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("amount", "", "Amount to fund (e.g. 10000basecro)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
