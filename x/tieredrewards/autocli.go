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
					RpcMethod:      "TierDelegate",
					Use:            "tier-delegate [position-id] [validator-address]",
					Short:          "Delegate a position's tokens to a validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}, {ProtoField: "validator_address"}},
				},
				{
					RpcMethod:      "TierUndelegate",
					Use:            "tier-undelegate [position-id]",
					Short:          "Begin undelegating a position's tokens from its validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}},
				},
				{
					RpcMethod:      "TierRedelegate",
					Use:            "tier-redelegate [position-id] [dst-validator-address]",
					Short:          "Move a position's delegation to a different validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "position_id"}, {ProtoField: "dst_validator_address"}},
				},
			},
		},
	}
}
