package cli

import (
	"fmt"
	"strings"

	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"
	"github.com/spf13/cobra"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/version"
)

// GetTxCmd returns the parent command for all x/inflation CLI tx commands.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Transactions commands for the inflation module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdUpdateParams())

	return cmd
}

// CmdUpdateParams returns a CLI command handler for MsgUpdateParams.
func CmdUpdateParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-params [max-supply] [decay-rate] [burned-addresses...]",
		Short: "Submit an update-params transaction for the inflation module",
		Long: fmt.Sprintf(`Submit an update-params transaction for the inflation module.

max-supply: maximum token supply (integer, 0 = unlimited)
decay-rate: annual decay rate as a decimal (e.g. 0.068 for 6.8%%)
burned-addresses: optional comma-separated list of burned addresses

Example:
$ %s tx inflation update-params 100000000000 0.068 --from authority
$ %s tx inflation update-params 100000000000 0.068 cro1addr1,cro1addr2 --from authority
`,
			version.AppName, version.AppName,
		),
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			maxSupply, decayRate, burnedAddresses, err := ParseUpdateParamsArgs(args)
			if err != nil {
				return err
			}

			authority := clientCtx.GetFromAddress().String()

			params := types.NewParams(maxSupply, burnedAddresses, decayRate)
			msg := &types.MsgUpdateParams{
				Authority: authority,
				Params:    params,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// ParseUpdateParamsArgs parses the positional arguments for the update-params command.
func ParseUpdateParamsArgs(args []string) (maxSupply sdkmath.Int, decayRate sdkmath.LegacyDec, burnedAddresses []string, err error) {
	maxSupply, ok := sdkmath.NewIntFromString(args[0])
	if !ok {
		return sdkmath.Int{}, sdkmath.LegacyDec{}, nil, fmt.Errorf("invalid max-supply: %s", args[0])
	}

	decayRate, err = sdkmath.LegacyNewDecFromStr(args[1])
	if err != nil {
		return sdkmath.Int{}, sdkmath.LegacyDec{}, nil, fmt.Errorf("invalid decay-rate: %w", err)
	}

	if len(args) == 3 && args[2] != "" {
		burnedAddresses = strings.Split(args[2], ",")
	}

	return maxSupply, decayRate, burnedAddresses, nil
}
