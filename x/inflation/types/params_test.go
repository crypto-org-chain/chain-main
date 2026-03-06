package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func TestNewParams_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      Params
		wantErr     bool
		errContains string
	}{
		{
			name: "valid typical config",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{},
				1,
				sdkmath.LegacyMustNewDecFromStr("0.0680"),
			),
			wantErr: false,
		},
		{
			name: "valid zero max supply (unlimited)",
			params: NewParams(
				sdkmath.NewInt(0),
				[]string{},
				1,
				sdkmath.LegacyZeroDec(),
			),
			wantErr: false,
		},
		{
			name: "valid with burned addresses",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{"cosmos1zpdh03ej2h9ct3lgjydqp3upqkktq322dcvwjm"},
				1,
				sdkmath.LegacyNewDecWithPrec(65, 3),
			),
			wantErr: false,
		},
		{
			name: "valid with multiple burned addresses",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{
					"cosmos1g69pjvgvdug5m9kphwh284rvls4g5jnrg4p8dm",
					"cosmos1ws268687q2xhu4gqwgwqhnwpthyt4td9t60nd8",
				},
				100,
				sdkmath.LegacyNewDecWithPrec(68, 3),
			),
			wantErr: false,
		},
		{
			name: "valid decay rate zero (disabled)",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{},
				1,
				sdkmath.LegacyZeroDec(),
			),
			wantErr: false,
		},
		{
			name: "valid decay rate one (100% decay)",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{},
				1,
				sdkmath.LegacyOneDec(),
			),
			wantErr: false,
		},
		{
			name:    "valid default params",
			params:  DefaultParams(),
			wantErr: false,
		},
		{
			name: "valid large max supply",
			params: NewParams(
				sdkmath.NewIntFromBigInt(sdkmath.NewInt(10_000_000_000).Mul(sdkmath.NewInt(100_000_000)).BigInt()),
				[]string{},
				1,
				sdkmath.LegacyMustNewDecFromStr("0.0680"),
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewParams_ValidateMaxSupply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      Params
		wantErr     bool
		errContains string
	}{
		{
			name: "negative max supply",
			params: NewParams(
				sdkmath.NewInt(-1000000000),
				[]string{},
				1,
				sdkmath.LegacyMustNewDecFromStr("0.0680"),
			),
			wantErr:     true,
			errContains: "max supply cannot be negative",
		},
		{
			name: "negative one max supply",
			params: NewParams(
				sdkmath.NewInt(-1),
				[]string{},
				1,
				sdkmath.LegacyZeroDec(),
			),
			wantErr:     true,
			errContains: "max supply cannot be negative",
		},
		{
			name: "zero max supply is valid (unlimited)",
			params: NewParams(
				sdkmath.NewInt(0),
				[]string{},
				1,
				sdkmath.LegacyZeroDec(),
			),
			wantErr: false,
		},
		{
			name: "positive max supply is valid",
			params: NewParams(
				sdkmath.NewInt(1),
				[]string{},
				1,
				sdkmath.LegacyZeroDec(),
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewParams_ValidateBurnedAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      Params
		wantErr     bool
		errContains string
	}{
		{
			name: "duplicate burned address",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{
					"cosmos139f7kncmglres2nf3h4hc4tade85ekfr8sulz5",
					"cosmos139f7kncmglres2nf3h4hc4tade85ekfr8sulz5",
				},
				1,
				sdkmath.LegacyMustNewDecFromStr("0.0680"),
			),
			wantErr:     true,
			errContains: "duplicate burned address",
		},
		{
			name: "empty burned address",
			params: NewParams(
				sdkmath.NewInt(1000000000),
				[]string{""},
				1,
				sdkmath.LegacyMustNewDecFromStr("0.0680"),
			),
			wantErr:     true,
			errContains: "invalid burned address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParams_ValidateDecayStartHeight(t *testing.T) {
	tests := []struct {
		name    string
		height  uint64
		wantErr bool
	}{
		{
			name:    "valid positive height",
			height:  1,
			wantErr: false,
		},
		{
			name:    "valid large height",
			height:  1000000,
			wantErr: false,
		},
		{
			name:    "invalid zero height",
			height:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := DefaultParams()
			params.DecayStartHeight = tt.height

			err := params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "decay start height must be positive")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParams_ValidateDecayRate(t *testing.T) {
	tests := []struct {
		name    string
		rate    sdkmath.LegacyDec
		wantErr bool
	}{
		{
			name:    "valid zero rate (disabled)",
			rate:    sdkmath.LegacyZeroDec(),
			wantErr: false,
		},
		{
			name:    "valid small rate",
			rate:    sdkmath.LegacyNewDecWithPrec(1, 2), // 0.01 = 1%
			wantErr: false,
		},
		{
			name:    "valid medium rate",
			rate:    sdkmath.LegacyNewDecWithPrec(65, 3), // 0.065 = 6.5%
			wantErr: false,
		},
		{
			name:    "valid one (100%)",
			rate:    sdkmath.LegacyOneDec(),
			wantErr: false,
		},
		{
			name:    "invalid negative rate",
			rate:    sdkmath.LegacyNewDecWithPrec(-1, 2),
			wantErr: true,
		},
		{
			name:    "invalid rate greater than one",
			rate:    sdkmath.LegacyNewDecWithPrec(101, 2), // 1.01 = 101%
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := DefaultParams()
			params.DecayRate = tt.rate

			err := params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "decay rate")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
