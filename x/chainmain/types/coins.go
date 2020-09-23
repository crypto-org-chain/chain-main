package types

import sdk "github.com/cosmos/cosmos-sdk/types"

type CoinParser interface {
	GetBaseUnit() string
	MustSprintBaseCoin(sdk.Int, string) string
	SprintBaseCoin(sdk.Int, string) (string, error)

	ParseCoin(string) (sdk.Coin, error)
	ParseCoinToa(string) (string, error)
	ParseCoins(string) (sdk.Coins, error)
	ParseCoinsToa(string) (string, error)
}
