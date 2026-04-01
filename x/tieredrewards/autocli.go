package tieredrewards

import (
	"fmt"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/cosmos/cosmos-sdk/version"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: "chainmain.tieredrewards.v1.Query",
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Query the current tieredrewards parameters",
				},
				{
					RpcMethod:      "TierPosition",
					Use:            "position [position-id]",
					Short:          "Query a single tier position by ID",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "TierPositionsByOwner",
					Use:            "positions-by-owner [owner]",
					Short:          "Query all tier positions for an owner address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "owner"}},
				},
				{
					RpcMethod: "AllTierPositions",
					Use:       "positions",
					Short:     "Query all tier positions (paginated)",
				},
				{
					RpcMethod: "Tiers",
					Use:       "tiers",
					Short:     "Query all tier definitions",
				},
				{
					RpcMethod: "TierPoolBalance",
					Use:       "pool-balance",
					Short:     "Query the bonus rewards pool balance",
				},
				{
					RpcMethod:      "EstimateTierRewards",
					Use:            "estimate-rewards [position-id]",
					Short:          "Estimate pending base and bonus rewards for a position",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "TierVotingPower",
					Use:            "voting-power [voter]",
					Short:          "Query governance voting power from delegated tier positions",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "voter"}},
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: "chainmain.tieredrewards.v1.Msg",
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod:      "UpdateParams",
					Use:            "update-params-proposal [params]",
					Short:          "Submit a proposal to update tieredrewards module params. Note: the entire params must be provided.",
					Long:           fmt.Sprintf("Submit a proposal to update tieredrewards module params. Note: the entire params must be provided.\n See the fields to fill in by running `%s query tieredrewards params --output json`", version.AppName),
					Example:        fmt.Sprintf(`%s tx tieredrewards update-params-proposal '{ "target_base_rewards_rate": "0.03" }'`, version.AppName),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "params"}},
					GovProposal:    true,
				},
				{
					RpcMethod:      "AddTier",
					Use:            "add-tier [tier]",
					Short:          "Create a new tier (authority-gated)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "tier"}},
					GovProposal:    true,
				},
				{
					RpcMethod:      "UpdateTier",
					Use:            "update-tier [tier]",
					Short:          "Update an existing tier (authority-gated)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "tier"}},
					GovProposal:    true,
				},
				{
					RpcMethod:      "DeleteTier",
					Use:            "delete-tier [id]",
					Short:          "Delete a tier (authority-gated)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "id"}},
					GovProposal:    true,
				},
				{
					RpcMethod:      "LockTier",
					Use:            "lock-tier [id] [amount] [validator-address]",
					Short:          "Lock tokens into a tier and delegate to a validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "id"}, {ProtoField: "amount"}, {ProtoField: "validator_address"}},
				},
				{
					RpcMethod:      "CommitDelegationToTier",
					Use:            "commit-delegation-to-tier [validator-address] [amount] [id]",
					Short:          "Commit an existing delegation to a tier position",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_address"}, {ProtoField: "amount"}, {ProtoField: "id"}},
				},
				{
					RpcMethod:      "TierDelegate",
					Use:            "tier-delegate [position-id] [validator]",
					Short:          "Delegate a position's tokens to a validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}, {ProtoField: "validator"}},
				},
				{
					RpcMethod:      "TierUndelegate",
					Use:            "tier-undelegate [position-id]",
					Short:          "Begin undelegating a position's tokens from its validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "TierRedelegate",
					Use:            "tier-redelegate [position-id] [dst-validator]",
					Short:          "Move a position's delegation to a different validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}, {ProtoField: "dst_validator"}},
				},
				{
					RpcMethod:      "AddToTierPosition",
					Use:            "add-to-tier-position [position-id] [amount]",
					Short:          "Add tokens to an existing position",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}, {ProtoField: "amount"}},
				},
				{
					RpcMethod:      "TriggerExitFromTier",
					Use:            "trigger-exit [position-id]",
					Short:          "Start the exit commitment for a position",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "ClearPosition",
					Use:            "clear-position [position-id]",
					Short:          "Clear exit state on a position so tokens can be added again",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "ClaimTierRewards",
					Use:            "claim-tier-rewards [position-id]",
					Short:          "Claim base and bonus rewards for a delegated position",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "WithdrawFromTier",
					Use:            "withdraw-from-tier [position-id]",
					Short:          "Withdraw locked tokens after exit commitment has elapsed",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "FundTierPool",
					Use:            "fund-tier-pool [amount]",
					Short:          "Fund the tier bonus rewards pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "amount"}},
				},
			},
		},
	}
}
