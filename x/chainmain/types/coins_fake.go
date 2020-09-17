package types

import (
	fmt "fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FakeCoinParser maintains string to sdk.Coin mapping for testing
type FakeCoinParser struct {
	mapping map[string]sdk.Coin
}

func NewFakeCoinParser(mapping map[string]sdk.Coin) *FakeCoinParser {
	return &FakeCoinParser{
		mapping,
	}
}

func (parser *FakeCoinParser) ParseCoin(s string) (sdk.Coin, error) {
	if coin, exist := parser.mapping[s]; exist {
		return coin, nil
	} else {
		return sdk.Coin{}, fmt.Errorf("Coin mapping not found for %s", s)
	}
}

func (parser *FakeCoinParser) ParseCoinToa(s string) (string, error) {
	if coin, err := parser.ParseCoin(s); err != nil {
		return "", err
	} else {
		return coin.String(), nil
	}
}

func (parser *FakeCoinParser) ParseCoins(s string) (sdk.Coins, error) {
	coinStrs := strings.Split(s, ",")
	coins := make(sdk.Coins, len(coinStrs))

	for i, coinStr := range coinStrs {
		coin, err := parser.ParseCoin(coinStr)
		if err != nil {
			return nil, err
		}

		coins[i] = coin
	}

	// sort coins for determinism
	coins.Sort()

	return coins, nil
}

func (parser *FakeCoinParser) ParseCoinsToa(s string) (string, error) {
	if coins, err := parser.ParseCoins(s); err != nil {
		return "", err
	} else {
		return coins.String(), nil
	}
}
