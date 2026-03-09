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
					Example:        fmt.Sprintf(`%s tx tieredrewards update-params-proposal '{ "target_base_reward_rate": "0.03" }'`, version.AppName),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "params"}},
					GovProposal:    true,
				},
			},
		},
	}
}
