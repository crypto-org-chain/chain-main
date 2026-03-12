package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func validTier() types.Tier {
	return types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24 * 365, // 1 year
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2), // 0.04
		MinLockAmount: sdkmath.NewInt(1000),
	}
}

func TestTier_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		modify      func(*types.Tier)
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid tier",
			modify: func(_ *types.Tier) {},
		},
		{
			name: "valid tier with close_only",
			modify: func(t *types.Tier) {
				t.CloseOnly = true
			},
		},
		{
			name: "valid tier with zero bonus apy",
			modify: func(t *types.Tier) {
				t.BonusApy = sdkmath.LegacyZeroDec()
			},
		},
		{
			name: "zero exit duration",
			modify: func(t *types.Tier) {
				t.ExitDuration = 0
			},
			wantErr:     true,
			errContains: "exit duration must be positive",
		},
		{
			name: "nil bonus apy",
			modify: func(t *types.Tier) {
				t.BonusApy = sdkmath.LegacyDec{}
			},
			wantErr:     true,
			errContains: "bonus apy cannot be nil",
		},
		{
			name: "negative bonus apy",
			modify: func(t *types.Tier) {
				t.BonusApy = sdkmath.LegacyNewDec(-1)
			},
			wantErr:     true,
			errContains: "bonus apy cannot be negative",
		},
		{
			name: "nil min lock amount",
			modify: func(t *types.Tier) {
				t.MinLockAmount = sdkmath.Int{}
			},
			wantErr:     true,
			errContains: "min lock amount cannot be nil",
		},
		{
			name: "zero min lock amount",
			modify: func(t *types.Tier) {
				t.MinLockAmount = sdkmath.ZeroInt()
			},
			wantErr:     true,
			errContains: "min lock amount must be positive",
		},
		{
			name: "negative min lock amount",
			modify: func(t *types.Tier) {
				t.MinLockAmount = sdkmath.NewInt(-100)
			},
			wantErr:     true,
			errContains: "min lock amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tier := validTier()
			tt.modify(&tier)
			err := tier.Validate()
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

func TestTier_IsCloseOnly(t *testing.T) {
	t.Parallel()

	tier := validTier()
	require.False(t, tier.IsCloseOnly())

	tier.CloseOnly = true
	require.True(t, tier.IsCloseOnly())
}
