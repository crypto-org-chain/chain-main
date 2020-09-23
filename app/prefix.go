package app

import (
	"flag"
	"log"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

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
	HumanCoinUnit          = "cro"
	BaseCoinUnit           = "basecro" // 10^-8 AKA "carson"
	CroExponent            = 8
	CoinToBaseUnitMuls     = map[string]uint64{
		"cro": 1_0000_0000,
	}
)

func SetConfig() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(AccountAddressPrefix, AccountPubKeyPrefix)
	config.SetBech32PrefixForValidator(ValidatorAddressPrefix, ValidatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(ConsNodeAddressPrefix, ConsNodePubKeyPrefix)

	config.SetCoinType(CoinType)
	config.SetFullFundraiserPath(FundraiserPath)

	croUnit := sdk.OneDec()
	err := sdk.RegisterDenom(HumanCoinUnit, croUnit)
	if err != nil {
		log.Fatal(err)
	}

	carsonUnit := sdk.NewDecWithPrec(1, int64(CroExponent)) // 10^-8 (carson)
	err = sdk.RegisterDenom(BaseCoinUnit, carsonUnit)

	if err != nil {
		log.Fatal(err)
	}

	config.Seal()
}

var testingConfigState = struct {
	mtx   sync.Mutex
	isSet bool
}{
	isSet: false,
}

func SetTestingConfig() {
	if !isGoTest() {
		panic("SetTestingConfig called but not running go test")
	}

	testingConfigState.mtx.Lock()
	defer testingConfigState.mtx.Unlock()

	if testingConfigState.isSet {
		return
	}

	SetConfig()

	testingConfigState.isSet = true
}

func isGoTest() bool {
	return flag.Lookup("test.v") != nil
}
