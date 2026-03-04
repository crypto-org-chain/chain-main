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
				sdkmath.LegacyNewDecWithPrec(680, 3),
			),
			wantErr:     false,
			errContains: "",
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
				sdkmath.LegacyNewDecWithPrec(680, 3),
			),
			wantErr:     true,
			errContains: "max supply cannot be negative",
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
				sdkmath.LegacyNewDecWithPrec(680, 3),
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
				sdkmath.LegacyNewDecWithPrec(680, 3),
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
