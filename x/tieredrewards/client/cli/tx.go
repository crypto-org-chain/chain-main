package cli

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govcli "github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

const flagTriggerExitImmediately = "trigger-exit-immediately"

// GetTxCmd returns the transaction commands for the tieredrewards module.
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "tieredrewards transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		GetCmdUpdateParamsProposal(),
		GetCmdAddTierProposal(),
		GetCmdUpdateTierProposal(),
		GetCmdDeleteTierProposal(),
		GetCmdLockTier(),
		GetCmdCommitDelegationToTier(),
		GetCmdTierUndelegate(),
		GetCmdTierRedelegate(),
		GetCmdAddToTierPosition(),
		GetCmdTriggerExitFromTier(),
		GetCmdClearPosition(),
		GetCmdClaimTierRewards(),
		GetCmdWithdrawFromTier(),
		GetCmdExitTierWithDelegation(),
	)

	return txCmd
}

func newTxCmd(
	use string,
	args cobra.PositionalArgs,
	short string,
	run func(client.Context, *cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Args:  args,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			return run(clientCtx, cmd, args)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func broadcastValidatedMsg(clientCtx client.Context, cmd *cobra.Command, msg validatingMsg) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
}

func newGovProposalCmd(
	use string,
	short string,
	buildInnerMsg func(client.Context, string) (sdk.Msg, error),
) *cobra.Command {
	cmd := newTxCmd(
		use,
		cobra.ExactArgs(1),
		short,
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			innerMsg, err := buildInnerMsg(clientCtx, args[0])
			if err != nil {
				return err
			}

			proposalMsg, err := buildGovProposalMsg(cmd, clientCtx, innerMsg)
			if err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), proposalMsg)
		},
	)

	addGovProposalFlags(cmd)
	return cmd
}

func newPositionTxCmd(
	use string,
	short string,
	buildMsg func(owner string, positionID uint64) validatingMsg,
) *cobra.Command {
	return newTxCmd(
		use,
		cobra.ExactArgs(1),
		short,
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			positionID, err := parseUint64Arg("position-id", args[0])
			if err != nil {
				return err
			}

			return broadcastValidatedMsg(clientCtx, cmd, buildMsg(clientCtx.GetFromAddress().String(), positionID))
		},
	)
}

func newPositionStringTxCmd(
	use string,
	short string,
	buildMsg func(owner string, positionID uint64, value string) validatingMsg,
) *cobra.Command {
	return newTxCmd(
		use,
		cobra.ExactArgs(2),
		short,
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			positionID, err := parseUint64Arg("position-id", args[0])
			if err != nil {
				return err
			}

			return broadcastValidatedMsg(clientCtx, cmd, buildMsg(clientCtx.GetFromAddress().String(), positionID, args[1]))
		},
	)
}

func GetCmdUpdateParamsProposal() *cobra.Command {
	return newGovProposalCmd(
		"update-params-proposal [params]",
		"Submit a proposal to update tieredrewards parameters",
		func(clientCtx client.Context, arg string) (sdk.Msg, error) {
			params, err := parseParamsArg(clientCtx, arg)
			if err != nil {
				return nil, err
			}

			return &types.MsgUpdateParams{
				Authority: govAuthorityAddress(),
				Params:    params,
			}, nil
		},
	)
}

func GetCmdAddTierProposal() *cobra.Command {
	return newGovProposalCmd(
		"add-tier [tier]",
		"Submit a proposal to create a new tier",
		func(clientCtx client.Context, arg string) (sdk.Msg, error) {
			tier, err := parseTierArg(clientCtx, arg)
			if err != nil {
				return nil, err
			}

			return &types.MsgAddTier{
				Authority: govAuthorityAddress(),
				Tier:      tier,
			}, nil
		},
	)
}

func GetCmdUpdateTierProposal() *cobra.Command {
	return newGovProposalCmd(
		"update-tier [tier]",
		"Submit a proposal to update an existing tier",
		func(clientCtx client.Context, arg string) (sdk.Msg, error) {
			tier, err := parseTierArg(clientCtx, arg)
			if err != nil {
				return nil, err
			}

			return &types.MsgUpdateTier{
				Authority: govAuthorityAddress(),
				Tier:      tier,
			}, nil
		},
	)
}

func GetCmdDeleteTierProposal() *cobra.Command {
	return newGovProposalCmd(
		"delete-tier [id]",
		"Submit a proposal to delete a tier",
		func(_ client.Context, arg string) (sdk.Msg, error) {
			tierID, err := parseUint32Arg("id", arg)
			if err != nil {
				return nil, err
			}

			return &types.MsgDeleteTier{
				Authority: govAuthorityAddress(),
				Id:        tierID,
			}, nil
		},
	)
}

func GetCmdLockTier() *cobra.Command {
	cmd := newTxCmd(
		"lock-tier [id] [amount] [validator-address]",
		cobra.ExactArgs(3),
		"Lock tokens into a tier and delegate to a validator",
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			tierID, err := parseUint32Arg("id", args[0])
			if err != nil {
				return err
			}

			amount, err := parseMathIntArg("amount", args[1])
			if err != nil {
				return err
			}

			triggerExitImmediately, err := cmd.Flags().GetBool(flagTriggerExitImmediately)
			if err != nil {
				return err
			}

			return broadcastValidatedMsg(clientCtx, cmd, &types.MsgLockTier{
				Owner:                  clientCtx.GetFromAddress().String(),
				Id:                     tierID,
				Amount:                 amount,
				ValidatorAddress:       args[2],
				TriggerExitImmediately: triggerExitImmediately,
			})
		},
	)

	cmd.Flags().Bool(flagTriggerExitImmediately, false, "Start the exit commitment immediately after lock")
	return cmd
}

func GetCmdCommitDelegationToTier() *cobra.Command {
	cmd := newTxCmd(
		"commit-delegation-to-tier [validator-address] [amount] [id]",
		cobra.ExactArgs(3),
		"Commit an existing delegation to a tier position",
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			amount, err := parseMathIntArg("amount", args[1])
			if err != nil {
				return err
			}

			tierID, err := parseUint32Arg("id", args[2])
			if err != nil {
				return err
			}

			triggerExitImmediately, err := cmd.Flags().GetBool(flagTriggerExitImmediately)
			if err != nil {
				return err
			}

			return broadcastValidatedMsg(clientCtx, cmd, &types.MsgCommitDelegationToTier{
				DelegatorAddress:       clientCtx.GetFromAddress().String(),
				ValidatorAddress:       args[0],
				Amount:                 amount,
				Id:                     tierID,
				TriggerExitImmediately: triggerExitImmediately,
			})
		},
	)

	cmd.Flags().Bool(flagTriggerExitImmediately, false, "Start the exit commitment immediately after commit")
	return cmd
}

func GetCmdTierUndelegate() *cobra.Command {
	return newPositionTxCmd(
		"tier-undelegate [position-id]",
		"Begin undelegating a position's tokens from its validator",
		func(owner string, positionID uint64) validatingMsg {
			return &types.MsgTierUndelegate{
				Owner:      owner,
				PositionId: positionID,
			}
		},
	)
}

func GetCmdTierRedelegate() *cobra.Command {
	return newPositionStringTxCmd(
		"tier-redelegate [position-id] [dst-validator]",
		"Move a position's delegation to a different validator",
		func(owner string, positionID uint64, dstValidator string) validatingMsg {
			return &types.MsgTierRedelegate{
				Owner:        owner,
				PositionId:   positionID,
				DstValidator: dstValidator,
			}
		},
	)
}

func GetCmdAddToTierPosition() *cobra.Command {
	return newTxCmd(
		"add-to-tier-position [position-id] [amount]",
		cobra.ExactArgs(2),
		"Add tokens to an existing position",
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			positionID, err := parseUint64Arg("position-id", args[0])
			if err != nil {
				return err
			}

			amount, err := parseMathIntArg("amount", args[1])
			if err != nil {
				return err
			}

			return broadcastValidatedMsg(clientCtx, cmd, &types.MsgAddToTierPosition{
				Owner:      clientCtx.GetFromAddress().String(),
				PositionId: positionID,
				Amount:     amount,
			})
		},
	)
}

func GetCmdTriggerExitFromTier() *cobra.Command {
	return newPositionTxCmd(
		"trigger-exit [position-id]",
		"Start the exit commitment for a position",
		func(owner string, positionID uint64) validatingMsg {
			return &types.MsgTriggerExitFromTier{
				Owner:      owner,
				PositionId: positionID,
			}
		},
	)
}

func GetCmdClearPosition() *cobra.Command {
	return newPositionTxCmd(
		"clear-position [position-id]",
		"Clear exit state on a position so tokens can be added again",
		func(owner string, positionID uint64) validatingMsg {
			return &types.MsgClearPosition{
				Owner:      owner,
				PositionId: positionID,
			}
		},
	)
}

func GetCmdClaimTierRewards() *cobra.Command {
	return newTxCmd(
		"claim-tier-rewards [position-id ...]",
		cobra.MinimumNArgs(1),
		"Claim base and bonus rewards for one or more positions",
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			positionIds := make([]uint64, len(args))
			for i, arg := range args {
				id, err := parseUint64Arg("position-id", arg)
				if err != nil {
					return err
				}
				positionIds[i] = id
			}

			return broadcastValidatedMsg(clientCtx, cmd, &types.MsgClaimTierRewards{
				Owner:       clientCtx.GetFromAddress().String(),
				PositionIds: positionIds,
			})
		},
	)
}

func GetCmdWithdrawFromTier() *cobra.Command {
	return newPositionTxCmd(
		"withdraw-from-tier [position-id]",
		"Withdraw locked tokens after exit commitment has elapsed",
		func(owner string, positionID uint64) validatingMsg {
			return &types.MsgWithdrawFromTier{
				Owner:      owner,
				PositionId: positionID,
			}
		},
	)
}

func GetCmdExitTierWithDelegation() *cobra.Command {
	return newTxCmd(
		"exit-tier-with-delegation [position-id] [amount]",
		cobra.ExactArgs(2),
		"Exit tier by transferring delegation back to owner (no unbonding period)",
		func(clientCtx client.Context, cmd *cobra.Command, args []string) error {
			positionID, err := parseUint64Arg("position-id", args[0])
			if err != nil {
				return err
			}

			amount, err := parseMathIntArg("amount", args[1])
			if err != nil {
				return err
			}

			return broadcastValidatedMsg(clientCtx, cmd, &types.MsgExitTierWithDelegation{
				Owner:      clientCtx.GetFromAddress().String(),
				PositionId: positionID,
				Amount:     amount,
			})
		},
	)
}

func addGovProposalFlags(cmd *cobra.Command) {
	cmd.Flags().String(govcli.FlagTitle, "", "Title of the governance proposal")
	cmd.Flags().String(govcli.FlagSummary, "", "Summary of the governance proposal")
	cmd.Flags().String(govcli.FlagDeposit, "", "Initial deposit for the governance proposal")
	cmd.Flags().String(govcli.FlagMetadata, "", "Metadata attached to the governance proposal")
	cmd.Flags().Bool(govcli.FlagExpedited, false, "Submit the governance proposal as expedited")

	mustMarkFlagRequired(cmd, govcli.FlagTitle)
	mustMarkFlagRequired(cmd, govcli.FlagSummary)
	mustMarkFlagRequired(cmd, govcli.FlagDeposit)
}

func buildGovProposalMsg(cmd *cobra.Command, clientCtx client.Context, innerMsg sdk.Msg) (*govv1.MsgSubmitProposal, error) {
	depositStr, err := cmd.Flags().GetString(govcli.FlagDeposit)
	if err != nil {
		return nil, err
	}

	deposit, err := sdk.ParseCoinsNormalized(depositStr)
	if err != nil {
		return nil, err
	}

	title, err := cmd.Flags().GetString(govcli.FlagTitle)
	if err != nil {
		return nil, err
	}

	summary, err := cmd.Flags().GetString(govcli.FlagSummary)
	if err != nil {
		return nil, err
	}

	metadata, err := cmd.Flags().GetString(govcli.FlagMetadata)
	if err != nil {
		return nil, err
	}

	expedited, err := cmd.Flags().GetBool(govcli.FlagExpedited)
	if err != nil {
		return nil, err
	}

	return govv1.NewMsgSubmitProposal(
		[]sdk.Msg{innerMsg},
		deposit,
		clientCtx.GetFromAddress().String(),
		metadata,
		title,
		summary,
		expedited,
	)
}

func govAuthorityAddress() string {
	return authtypes.NewModuleAddress(govtypes.ModuleName).String()
}

func parseParamsArg(clientCtx client.Context, arg string) (types.Params, error) {
	var params types.Params
	if err := unmarshalJSONArg(clientCtx, arg, &params); err != nil {
		return types.Params{}, err
	}

	if err := params.Validate(); err != nil {
		return types.Params{}, err
	}

	return params, nil
}

func parseTierArg(clientCtx client.Context, arg string) (types.Tier, error) {
	var tier types.Tier
	if err := unmarshalJSONArg(clientCtx, arg, &tier); err != nil {
		return types.Tier{}, err
	}

	if err := tier.Validate(); err != nil {
		return types.Tier{}, err
	}

	return tier, nil
}
