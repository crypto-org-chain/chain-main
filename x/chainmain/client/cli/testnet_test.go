package cli

import (
	"testing"

	"github.com/stretchr/testify/suite"

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
	coins := sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 11), sdk.NewInt64Coin(otherDenom, 4))

	testCases := []struct {
		name          string
		stakingAmount string
		expCoin       sdk.Coin
		expErr        string
	}{
		{
			name:    "empty amount stakes half of each coin in the base denom",
			expCoin: sdk.NewInt64Coin(baseDenom, 7),
		},
		{
			name:          "explicit amount",
			stakingAmount: "3" + baseDenom,
			expCoin:       sdk.NewInt64Coin(baseDenom, 3),
		},
		{
			name:          "invalid amount",
			stakingAmount: "3 3",
			expErr:        "failed to parse staking coin",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			coin, err := parseStakingCoin(baseDenom, coins, tc.stakingAmount)
			if tc.expErr != "" {
				s.Require().ErrorContains(err, tc.expErr)
				return
			}
			s.Require().NoError(err)
			s.Require().Equal(tc.expCoin, coin)
		})
	}
}
