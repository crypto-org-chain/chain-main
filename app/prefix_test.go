package app_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-com/chain-main/app"
	"github.com/stretchr/testify/require"
)

func TestConversion(t *testing.T) {
	app.SetConfig()

	testCases := []struct {
		input  sdk.Coin
		denom  string
		result sdk.Coin
		expErr bool
	}{
		{sdk.NewCoin("foo", sdk.ZeroInt()), app.HumanCoinUnit, sdk.Coin{}, true},
		{sdk.NewCoin(app.HumanCoinUnit, sdk.ZeroInt()), "foo", sdk.Coin{}, true},
		{sdk.NewCoin(app.HumanCoinUnit, sdk.ZeroInt()), "FOO", sdk.Coin{}, true},

		{sdk.NewCoin(app.HumanCoinUnit, sdk.NewInt(5)),
			app.BaseCoinUnit, sdk.NewCoin(app.BaseCoinUnit, sdk.NewInt(500000000)), false}, // cro => carson

		{sdk.NewCoin(app.BaseCoinUnit, sdk.NewInt(500000000)),
			app.HumanCoinUnit, sdk.NewCoin(app.HumanCoinUnit, sdk.NewInt(5)), false}, // carson => cro

	}

	for i, tc := range testCases {
		res, err := sdk.ConvertCoin(tc.input, tc.denom)
		require.Equal(
			t, tc.expErr, err != nil,
			"unexpected error; tc: #%d, input: %s, denom: %s", i+1, tc.input, tc.denom,
		)
		require.Equal(
			t, tc.result, res,
			"invalid result; tc: #%d, input: %s, denom: %s", i+1, tc.input, tc.denom,
		)
	}

}
