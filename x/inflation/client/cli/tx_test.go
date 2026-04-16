package cli_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/inflation/client/cli"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

// TestCmdUpdateParams_ParsesDecimalFormats verifies that the update-params
// command correctly parses decay_rate in both short ("0.068") and full-precision
// ("0.068000000000000000") formats. The full-precision format is what the
// query command returns, so it must be accepted as input.
func TestCmdUpdateParams_ParsesDecimalFormats(t *testing.T) {
	testCases := []struct {
		name      string
		args      []string
		expErr    bool
		expErrMsg string
		expSupply sdkmath.Int
		expDecay  sdkmath.LegacyDec
		expBurned int
	}{
		{
			name:      "short decimal format",
			args:      []string{"100000000000", "0.068"},
			expSupply: sdkmath.NewInt(100000000000),
			expDecay:  sdkmath.LegacyMustNewDecFromStr("0.068"),
		},
		{
			name:      "full-precision format from query output",
			args:      []string{"100000000000", "0.068000000000000000"},
			expSupply: sdkmath.NewInt(100000000000),
			expDecay:  sdkmath.LegacyMustNewDecFromStr("0.068"),
		},
		{
			name:      "zero decay rate",
			args:      []string{"0", "0"},
			expSupply: sdkmath.NewInt(0),
			expDecay:  sdkmath.LegacyZeroDec(),
		},
		{
			name:      "max decay rate",
			args:      []string{"0", "1.000000000000000000"},
			expSupply: sdkmath.NewInt(0),
			expDecay:  sdkmath.LegacyOneDec(),
		},
		{
			name:      "with burned addresses",
			args:      []string{"100000000000", "0.068", "cosmos139f7kncmglres2nf3h4hc4tade85ekfr8sulz5"},
			expSupply: sdkmath.NewInt(100000000000),
			expDecay:  sdkmath.LegacyMustNewDecFromStr("0.068"),
			expBurned: 1,
		},
		{
			name:      "invalid max-supply",
			args:      []string{"not_a_number", "0.068"},
			expErr:    true,
			expErrMsg: "invalid max-supply",
		},
		{
			name:      "invalid decay-rate (not a decimal)",
			args:      []string{"100000000000", "abc"},
			expErr:    true,
			expErrMsg: "invalid decay-rate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			maxSupply, decayRate, burnedAddresses, err := cli.ParseUpdateParamsArgs(tc.args)
			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrMsg)
				return
			}
			require.NoError(t, err)
			require.True(t, tc.expSupply.Equal(maxSupply), "max-supply mismatch: expected %s, got %s", tc.expSupply, maxSupply)
			require.True(t, tc.expDecay.Equal(decayRate), "decay-rate mismatch: expected %s, got %s", tc.expDecay, decayRate)
			require.Equal(t, tc.expBurned, len(burnedAddresses))
		})
	}
}
