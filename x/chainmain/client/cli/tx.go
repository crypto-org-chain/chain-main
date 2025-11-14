package cli

import (
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/chainmain/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	chainmainTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	flags.AddTxFlagsToCmd(chainmainTxCmd)

	return chainmainTxCmd
}
