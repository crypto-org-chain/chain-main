// +build devnet

package app

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
	HumanCoinUnit          = "cro"
	BaseCoinUnit           = "basecro" // 10^-8 AKA "carson"
	CroExponent            = 8
	CoinToBaseUnitMuls     = map[string]uint64{
		"cro": 1_0000_0000,
	}
)
