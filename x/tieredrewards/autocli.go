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
					RpcMethod: "TierPoolBalance",
					Use:       "tier-pool-balance",
					Short:     "Query the balance of the tier rewards pool",
				},
				{
					RpcMethod:      "TierPosition",
					Use:            "tier-position [position-id]",
					Short:          "Query a tier position by ID",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "TierPositionsByOwner",
					Use:            "tier-positions-by-owner [owner]",
					Short:          "Query all tier positions for an owner",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "owner"}},
				},
				{
					RpcMethod: "AllTierPositions",
					Use:       "all-tier-positions",
					Short:     "Query all tier positions",
				},
				{
					RpcMethod:      "EstimateTierBonus",
					Use:            "estimate-tier-bonus [position-id]",
					Short:          "Estimate pending bonus rewards for a tier position",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "TierVotingPower",
					Use:            "tier-voting-power [owner]",
					Short:          "Query the tier voting power for an address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "owner"}},
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              "chainmain.tieredrewards.v1.Msg",
			EnhanceCustomCommand: true,
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
				// LockTier, CommitDelegationToTier, AddToTierPosition, and FundTierPool
				// use cosmos.base.v1beta1.Coin fields which trigger a proto.Merge panic
				// in autocli v2 (dynamicpb/pulsar field descriptor mismatch).
				// These are handled by custom Cobra commands in client/cli/tx.go.
				{
					RpcMethod: "LockTier",
					Skip:      true,
				},
				{
					RpcMethod: "CommitDelegationToTier",
					Skip:      true,
				},
				{
					RpcMethod: "AddToTierPosition",
					Skip:      true,
				},
				{
					RpcMethod: "FundTierPool",
					Skip:      true,
				},
				{
					RpcMethod: "TierDelegate",
					Use:       "tier-delegate",
					Short:     "Delegate a tier position's tokens to a validator",
				},
				{
					RpcMethod: "TierUndelegate",
					Use:       "tier-undelegate",
					Short:     "Undelegate a tier position (only allowed after exit triggered)",
				},
				{
					RpcMethod: "TierRedelegate",
					Use:       "tier-redelegate",
					Short:     "Redelegate a tier position to a different validator",
				},
				{
					RpcMethod: "TriggerExitFromTier",
					Use:       "trigger-exit-from-tier",
					Short:     "Start the exit commitment for a tier position",
				},
				{
					RpcMethod: "WithdrawFromTier",
					Use:       "withdraw-from-tier",
					Short:     "Claim tokens after the exit commitment has elapsed",
				},
				{
					RpcMethod: "WithdrawTierRewards",
					Use:       "withdraw-tier-rewards",
					Short:     "Withdraw base and bonus rewards for a tier position",
				},
				{
					RpcMethod: "TransferTierPosition",
					Use:       "transfer-tier-position",
					Short:     "Transfer ownership of a tier position",
				},
			},
		},
	}
}
