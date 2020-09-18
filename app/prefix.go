package app

import (
	"log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chain "github.com/crypto-com/chain-main/x/chainmain"
)

func SetConfig() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(chain.AccountAddressPrefix, chain.AccountPubKeyPrefix)
	config.SetBech32PrefixForValidator(chain.ValidatorAddressPrefix, chain.ValidatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(chain.ConsNodeAddressPrefix, chain.ConsNodePubKeyPrefix)

	config.SetCoinType(chain.CoinType)
	config.SetFullFundraiserPath(chain.FundraiserPath)

	croUnit := sdk.OneDec()
	err := sdk.RegisterDenom(chain.HumanCoinUnit, croUnit)
	if err != nil {
		log.Fatal(err)
	}

	carsonUnit := sdk.NewDecWithPrec(1, int64(chain.CroExponential)) // 10^-8 (carson)
	err = sdk.RegisterDenom(chain.BaseCoinUnit, carsonUnit)

	if err != nil {
		log.Fatal(err)
	}

	config.Seal()
}
