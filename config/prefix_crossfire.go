// +build !testnet,crossfire

package config

const (
	CoinType       = 394
	FundraiserPath = "44'/394'/0'/0/1"
)

var (
	AccountAddressPrefix   = "cro"
	AccountPubKeyPrefix    = "cropub"
	ValidatorAddressPrefix = "crocncl"
	ValidatorPubKeyPrefix  = "crocnclpub"
	ConsNodeAddressPrefix  = "crocnclcons"
	ConsNodePubKeyPrefix   = "crocnclconspub"
	HumanCoinUnit          = "tcro"
	BaseCoinUnit           = "basetcro" // 10^-8 AKA "carson"
	CroExponent            = 8
)
