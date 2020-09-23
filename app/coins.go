package app

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SDKCoinParser use CosmosSDK denom functionality to perform coin conversion
type SDKCoinParser struct {
	baseUnit string

	coinToBaseUnitMuls map[string]uint64
}

func NewSDKCoinParser(baseUnit string, coinToBaseUnitMuls map[string]uint64) *SDKCoinParser {
	return &SDKCoinParser{
		baseUnit,

		coinToBaseUnitMuls,
	}
}

func (parser *SDKCoinParser) GetBaseUnit() string {
	return parser.baseUnit
}

func (parser *SDKCoinParser) MustSprintBaseCoin(baseCoin sdk.Int, denom string) string {
	result, err := parser.SprintBaseCoin(baseCoin, denom)
	if err != nil {
		panic(err)
	}

	return result
}

func (parser *SDKCoinParser) SprintBaseCoin(baseCoin sdk.Int, denom string) (string, error) {
	mul, exist := parser.coinToBaseUnitMuls[denom]
	if !exist {
		return "", fmt.Errorf("invalid coin denom: %s", denom)
	}

	return fmt.Sprintf("%d.%08d", baseCoin.Uint64()/mul, baseCoin.Uint64()%mul), nil
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
