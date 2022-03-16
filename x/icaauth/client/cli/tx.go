package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"github.com/spf13/cobra"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdRegisterAccount())
	cmd.AddCommand(CmdSubmitTx())

	return cmd
}
