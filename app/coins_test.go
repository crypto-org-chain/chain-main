package app_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-com/chain-main/app"
	"github.com/crypto-com/chain-main/config"
)

func TestParseCoin(t *testing.T) {
	config.SetTestingConfig()

	assert := assert.New(t)

	coinParser := app.NewSDKCoinParser("basecro", config.CoinToBaseUnitMuls)

	var coin sdk.Coin
	var err error

	_, err = coinParser.ParseCoin("invalid")
	assert.Error(err, "invalid coin expression: invalid")

	_, err = coinParser.ParseCoin("1000invalid")
	assert.Error(err, "source denom not registered: invalid")

	coin, err = coinParser.ParseCoin("1000cro")
	assert.Equal(sdk.NewCoin("basecro", sdk.NewInt(100000000000)), coin)
	assert.Nil(err)

	_, err = coinParser.ParseCoin("1000cro,1000cro")
	assert.Error(err, "invalid coin expression: 1000cro,1000cro")
}

func TestParseCoins(t *testing.T) {
	config.SetTestingConfig()

	assert := assert.New(t)

	coinParser := app.NewSDKCoinParser("basecro", config.CoinToBaseUnitMuls)

	var coins sdk.Coins
	var err error

	_, err = coinParser.ParseCoins("invalid")
	assert.Error(err, "invalid coin expression: invalid")

	_, err = coinParser.ParseCoins("1000invalid")
	assert.Error(err, "source denom not registered: invalid")

	coins, err = coinParser.ParseCoins("1000cro")
	assert.Equal(sdk.Coins{
		sdk.NewCoin("basecro", sdk.NewInt(100000000000)),
	}, coins)
	assert.Nil(err)

	_, err = coinParser.ParseCoins("1000cro,1000cro")
	assert.Error(err, "duplicate denomination cro")

	coins, err = coinParser.ParseCoins("1000cro,2000basecro")
	assert.Equal(sdk.Coins{
		sdk.NewCoin("basecro", sdk.NewInt(2000)),
		sdk.NewCoin("basecro", sdk.NewInt(100000000000)),
	}, coins)
	assert.Nil(err)
}

func TestSprintBaseCoin(t *testing.T) {
	config.SetTestingConfig()

	assert := assert.New(t)

	coinParser := app.NewSDKCoinParser("basecro", config.CoinToBaseUnitMuls)

	var coin string
	var err error

	_, err = coinParser.SprintBaseCoin(sdk.NewInt(1), "invalid")
	assert.Error(err, "invalid coin expression: invalid")

	coin, err = coinParser.SprintBaseCoin(sdk.NewInt(1), "cro")
	assert.Equal("0.00000001", coin)
	assert.Nil(err)

	totalSupply, _ := new(big.Int).SetString("100_000_000_000_0000_0000", 0)
	coin, err = coinParser.SprintBaseCoin(sdk.NewIntFromBigInt(totalSupply), "cro")
	assert.Equal("100000000000.00000000", coin)
	assert.Nil(err)
}
