package gov

import (
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/spf13/cobra"

	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
	"github.com/crypto-com/chain-main/x/gov/client/cli"
)

type AppModuleBasic struct {
	gov.AppModuleBasic

	coinParser       chainsdk.CoinParser
	proposalHandlers []govclient.ProposalHandler // proposal handlers which live in governance cli and rest
}

func NewAppModuleBasic(coinParser chainsdk.CoinParser, proposalHandlers ...govclient.ProposalHandler) AppModuleBasic {
	return AppModuleBasic{
		coinParser:       coinParser,
		proposalHandlers: proposalHandlers,
	}
}

// GetTxCmd returns the root tx command for the gov module.
func (module AppModuleBasic) GetTxCmd() *cobra.Command {
	proposalCLIHandlers := make([]*cobra.Command, 0, len(module.proposalHandlers))
	for _, proposalHandler := range module.proposalHandlers {
		proposalCLIHandlers = append(proposalCLIHandlers, proposalHandler.CLIHandler())
	}

	return cli.NewTxCmd(proposalCLIHandlers, module.coinParser)
}
