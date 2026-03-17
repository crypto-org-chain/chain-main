package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestParams_Validate(t *testing.T) {
	t.Parallel()

	validAddr := sdk.AccAddress([]byte("valid_funder________")).String()
	validAddr2 := sdk.AccAddress([]byte("valid_funder2_______")).String()

	tests := []struct {
		name        string
		params      types.Params
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid zero rate",
			params:  types.NewParams(sdkmath.LegacyZeroDec(), nil),
			wantErr: false,
		},
		{
			name:    "valid 3% rate",
			params:  types.NewParams(sdkmath.LegacyNewDecWithPrec(3, 2), nil),
			wantErr: false,
		},
		{
			name:    "valid 100% rate",
			params:  types.NewParams(sdkmath.LegacyOneDec(), nil),
			wantErr: false,
		},
		{
			name:    "valid large rate",
			params:  types.NewParams(sdkmath.LegacyNewDec(10), nil),
			wantErr: false,
		},
		{
			name:        "negative rate",
			params:      types.NewParams(sdkmath.LegacyNewDec(-1), nil),
			wantErr:     true,
			errContains: "negative",
		},
		{
			name:        "nil rate",
			params:      types.Params{},
			wantErr:     true,
			errContains: "nil",
		},
		{
			name:    "valid with pool funders",
			params:  types.NewParams(sdkmath.LegacyZeroDec(), []string{validAddr}),
			wantErr: false,
		},
		{
			name:    "valid with multiple pool funders",
			params:  types.NewParams(sdkmath.LegacyZeroDec(), []string{validAddr, validAddr2}),
			wantErr: false,
		},
		{
			name:    "valid with empty pool funders",
			params:  types.NewParams(sdkmath.LegacyZeroDec(), []string{}),
			wantErr: false,
		},
		{
			name:        "invalid pool funder address",
			params:      types.NewParams(sdkmath.LegacyZeroDec(), []string{"not-a-valid-address"}),
			wantErr:     true,
			errContains: "invalid pool funder address",
		},
		{
			name:        "duplicate pool funder",
			params:      types.NewParams(sdkmath.LegacyZeroDec(), []string{validAddr, validAddr}),
			wantErr:     true,
			errContains: "duplicate pool funder",
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
	require.Nil(t, params.PoolFunders)
	require.NoError(t, params.Validate())
}

func TestParams_IsAuthorizedFunder(t *testing.T) {
	addr1 := sdk.AccAddress([]byte("funder1_____________")).String()
	addr2 := sdk.AccAddress([]byte("funder2_____________")).String()
	addr3 := sdk.AccAddress([]byte("outsider____________")).String()

	params := types.NewParams(sdkmath.LegacyZeroDec(), []string{addr1, addr2})

	require.True(t, params.IsAuthorizedFunder(addr1))
	require.True(t, params.IsAuthorizedFunder(addr2))
	require.False(t, params.IsAuthorizedFunder(addr3))

	emptyParams := types.NewParams(sdkmath.LegacyZeroDec(), nil)
	require.False(t, emptyParams.IsAuthorizedFunder(addr1))
}
