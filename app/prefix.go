package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	AccountAddressPrefix   = "cro"
	AccountPubKeyPrefix    = "cropub"
	ValidatorAddressPrefix = "crovaloper"
	ValidatorPubKeyPrefix  = "crovaloperpub"
	ConsNodeAddressPrefix  = "crovalcons"
	ConsNodePubKeyPrefix   = "crovalconspub"
)

func SetConfig() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(AccountAddressPrefix, AccountPubKeyPrefix)
	config.SetBech32PrefixForValidator(ValidatorAddressPrefix, ValidatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(ConsNodeAddressPrefix, ConsNodePubKeyPrefix)
	config.Seal()
}
