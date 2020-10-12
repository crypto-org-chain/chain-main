// +build devnet

package config

const (
	CoinType       = 394
	FundraiserPath = "44'/1'/0'/0/1"
)

var (
	AccountAddressPrefix   = "dcro"
	AccountPubKeyPrefix    = "dcropub"
	ValidatorAddressPrefix = "dcrocncl"
	ValidatorPubKeyPrefix  = "dcrocnclpub"
	ConsNodeAddressPrefix  = "dcrocnclcons"
	ConsNodePubKeyPrefix   = "dcrocnclconspub"
	HumanCoinUnit          = "dcro"
	BaseCoinUnit           = "basedcro" // 10^-8 AKA "carson"
	CroExponent            = 8
	CoinToBaseUnitMuls     = map[string]uint64{
		"dcro": 1_0000_0000,
	}
)
