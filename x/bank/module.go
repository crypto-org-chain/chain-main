package bank

import (
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/spf13/cobra"

	"github.com/crypto-com/chain-main/x/bank/client/cli"
	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
)

type AppModuleBasic struct {
	bank.AppModuleBasic

	coinParser chainsdk.CoinParser
}

func NewAppModuleBasic(coinParser chainsdk.CoinParser) AppModuleBasic {
	return AppModuleBasic{
		coinParser: coinParser,
	}
}

// GetTxCmd returns the root tx command for the bank module.
func (module AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd(module.coinParser)
}
