package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type CoinParser interface {
	ParseCoin(string) (sdk.Coin, error)
	ParseCoinToa(string) (string, error)
	ParseCoins(string) (sdk.Coins, error)
	ParseCoinsToa(string) (string, error)
}

// SDKCoinParser use CosmosSDK denom functionality to perform coin conversion
type SDKCoinParser struct {
	baseUnit string
}

func NewSDKCoinParser(baseUnit string) *SDKCoinParser {
	return &SDKCoinParser{
		baseUnit,
	}
}

func (parser *SDKCoinParser) ParseCoin(s string) (sdk.Coin, error) {
	coin, err := sdk.ParseCoin(s)
	if err != nil {
		return sdk.Coin{}, err
	}
	coin, err = sdk.ConvertCoin(coin, parser.baseUnit)
	if err != nil {
		return sdk.Coin{}, err
	}
	return coin, nil
}

func (parser *SDKCoinParser) ParseCoinToa(s string) (string, error) {
	coin, err := parser.ParseCoin(s)
	if err != nil {
		return "", err
	}
	return coin.String(), nil
}

func (parser *SDKCoinParser) ParseCoins(s string) (sdk.Coins, error) {
	coins, err := sdk.ParseCoins(s)
	if err != nil {
		return []sdk.Coin{}, err
	}
	for i, coin := range coins {
		coins[i], err = sdk.ConvertCoin(coin, parser.baseUnit)
		if err != nil {
			return []sdk.Coin{}, err
		}
	}
	return coins, nil
}

func (parser *SDKCoinParser) ParseCoinsToa(s string) (string, error) {
	coins, err := parser.ParseCoins(s)
	if err != nil {
		return "", err
	}
	return coins.String(), nil
}
