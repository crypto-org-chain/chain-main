package cli

import (
	"testing"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestnetTestSuite struct {
	suite.Suite
}

func TestTestnetTestSuite(t *testing.T) {
	suite.Run(t, new(TestnetTestSuite))
}

func (s *TestnetTestSuite) TestParseStakingCoin() {
	const (
		baseDenom  = "basecro"
		otherDenom = "stake"
	)
	registerBaseDenom := func() {
		if _, ok := sdk.GetDenomUnit(baseDenom); !ok {
			s.Require().NoError(sdk.RegisterDenom(baseDenom, sdkmath.LegacyOneDec()))
		}
	}
	coins := sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 11), sdk.NewInt64Coin(otherDenom, 4))

	testCases := []struct {
		name          string
		malleate      func()
		stakingAmount string
		expCoin       sdk.Coin
		expErr        string
	}{
		{
			name: "no base denom registered",
			// Denom registration is process-wide and cannot be undone, so this case runs first and only once.
			malleate: func() {
				if _, err := sdk.GetBaseDenom(); err == nil {
					s.T().Skip("a base denom is already registered")
				}
			},
			expErr: "failed to get base denom",
		},
		{
			name:     "empty amount stakes half of each coin in the base denom",
			malleate: registerBaseDenom,
			expCoin:  sdk.NewInt64Coin(baseDenom, 7),
		},
		{
			name:          "explicit amount",
			malleate:      registerBaseDenom,
			stakingAmount: "3" + baseDenom,
			expCoin:       sdk.NewInt64Coin(baseDenom, 3),
		},
		{
			name:          "invalid amount",
			malleate:      registerBaseDenom,
			stakingAmount: "3 3",
			expErr:        "failed to parse staking coin",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			tc.malleate()

			coin, err := parseStakingCoin(coins, tc.stakingAmount)
			if tc.expErr != "" {
				s.Require().ErrorContains(err, tc.expErr)
				return
			}
			s.Require().NoError(err)
			s.Require().Equal(tc.expCoin, coin)
		})
	}
}
