package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func TestParams_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      types.Params
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid zero rate",
			params:  types.NewParams(sdkmath.LegacyZeroDec(), []types.TierDefinition{}),
			wantErr: false,
		},
		{
			name:    "valid 3% rate",
			params:  types.NewParams(sdkmath.LegacyNewDecWithPrec(3, 2), []types.TierDefinition{}),
			wantErr: false,
		},
		{
			name:    "valid 100% rate",
			params:  types.NewParams(sdkmath.LegacyOneDec(), []types.TierDefinition{}),
			wantErr: false,
		},
		{
			name:    "valid large rate",
			params:  types.NewParams(sdkmath.LegacyNewDec(10), []types.TierDefinition{}),
			wantErr: false,
		},
		{
			name:        "negative rate",
			params:      types.NewParams(sdkmath.LegacyNewDec(-1), []types.TierDefinition{}),
			wantErr:     true,
			errContains: "negative",
		},
		{
			name:        "nil rate",
			params:      types.Params{},
			wantErr:     true,
			errContains: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()
	require.True(t, params.TargetBaseRewardsRate.IsZero())
	require.NoError(t, params.Validate())
}
