package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/crypto-com/chain-main/app"
)

func TestParseCoin(t *testing.T) {
	app.SetConfig()

	assert := assert.New(t)

	assert.PanicsWithError(
		"invalid coin expression: invalid",
		func() {
			app.ParseCoin("invalid")
		},
	)

	assert.PanicsWithError(
		"source denom not registered: invalid",
		func() {
			app.ParseCoin("1000invalid")
		},
	)

	assert.Equal("100000000000basecro", app.ParseCoin("1000cro"))

	assert.PanicsWithError(
		"invalid coin expression: 1000cro,1000cro",
		func() {
			app.ParseCoin("1000cro,1000cro")
		},
	)
}

func TestParseCoins(t *testing.T) {
	app.SetConfig()

	assert := assert.New(t)

	assert.PanicsWithError(
		"invalid coin expression: invalid",
		func() {
			app.ParseCoins("invalid")
		},
	)

	assert.PanicsWithError(
		"source denom not registered: invalid",
		func() {
			app.ParseCoins("1000invalid")
		},
	)

	assert.Equal("100000000000basecro", app.ParseCoins("1000cro"))

	assert.PanicsWithError(
		"duplicate denomination cro",
		func() {
			app.ParseCoins("1000cro,1000cro")
		},
	)

	assert.Equal("2000basecro,100000000000basecro", app.ParseCoins("1000cro,2000basecro"))
}
