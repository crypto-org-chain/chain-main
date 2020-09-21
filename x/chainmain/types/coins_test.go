package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-com/chain-main/app"
	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
)

func TestParseCoin(t *testing.T) {
	app.SetConfig()

	assert := assert.New(t)

	coinParser := chainsdk.NewSDKCoinParser("basecro")

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
	assert := assert.New(t)

	coinParser := chainsdk.NewSDKCoinParser("basecro")

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
