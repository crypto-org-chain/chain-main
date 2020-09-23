package staking

import (
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/spf13/cobra"

	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
	"github.com/crypto-com/chain-main/x/staking/client/cli"
)

// AppModuleBasic defines the basic application module used by the staking module.
type AppModuleBasic struct {
	staking.AppModuleBasic

	coinParser chainsdk.CoinParser
}

func NewAppModuleBasic(coinParser chainsdk.CoinParser) AppModuleBasic {
	return AppModuleBasic{
		coinParser: coinParser,
	}
}

// GetTxCmd returns the root tx command for the staking module.
func (module AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd(module.coinParser)
}
