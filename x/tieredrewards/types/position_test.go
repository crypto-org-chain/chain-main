package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	testOwner     = sdk.AccAddress([]byte("test_owner__________")).String()
	testValidator = sdk.ValAddress([]byte("test_validator______")).String()
)

func validPosition() types.Position {
	return types.NewPosition(1, testOwner, 1, sdkmath.NewInt(1000), 100, types.Delegation{
		Validator:           testValidator,
		Shares:              sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{},
	}, time.Now())
}

func TestPosition_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		modify      func(*types.Position)
		wantErr     bool
		errContains string
	}{
		{
			name: "valid delegated position",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Validator: testValidator,
					Shares:    sdkmath.LegacyNewDec(1000),
					BaseRewardsPerShare: sdk.DecCoins{
						sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
					},
				}, time.Now())
			},
		},
		{
			name: "valid undelegated position with exit triggered",
			modify: func(p *types.Position) {
				p.TriggerExit(time.Now(), time.Hour*24*365)
				p.ClearDelegation()
			},
		},
		{
			name: "valid undelegated position with zero amount (slashed redelegation)",
			modify: func(p *types.Position) {
				p.ClearDelegation()
				p.UpdateAmount(sdkmath.ZeroInt())
			},
		},
		{
			name: "invalid undelegated position - non-zero amount without exit",
			modify: func(p *types.Position) {
				p.ClearDelegation()
			},
			wantErr:     true,
			errContains: "undelegated position must have exit triggered or zero amount",
		},
		{
			name: "valid exiting position",
			modify: func(p *types.Position) {
				now := time.Now()
				p.TriggerExit(now, time.Hour*24*365)
			},
		},
		{
			name: "empty owner",
			modify: func(p *types.Position) {
				p.Owner = ""
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "invalid owner address",
			modify: func(p *types.Position) {
				p.Owner = "not_a_valid_address"
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "nil amount locked",
			modify: func(p *types.Position) {
				p.UpdateAmount(sdkmath.Int{})
			},
			wantErr:     true,
			errContains: "amount locked cannot be nil",
		},
		{
			name: "zero amount locked is valid (post-slash state)",
			modify: func(p *types.Position) {
				p.UpdateAmount(sdkmath.ZeroInt())
			},
		},
		{
			name: "negative amount locked",
			modify: func(p *types.Position) {
				p.UpdateAmount(sdkmath.NewInt(-500))
			},
			wantErr:     true,
			errContains: "must not be negative",
		},
		{
			name: "negative delegated shares when delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Validator: testValidator,
					Shares:    sdkmath.LegacyNewDec(-1),
					BaseRewardsPerShare: sdk.DecCoins{
						sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
					},
				}, time.Now())
			},
			wantErr:     true,
			errContains: "delegated shares must be positive when validator is set",
		},
		{
			name: "invalid validator address when delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Validator: "not_valid",
					Shares:    sdkmath.LegacyNewDec(100),
					BaseRewardsPerShare: sdk.DecCoins{
						sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
					},
				}, time.Now())
			},
			wantErr:     true,
			errContains: "invalid validator address",
		},
		{
			name: "zero delegated shares when delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Validator: testValidator,
					Shares:    sdkmath.LegacyZeroDec(),
					BaseRewardsPerShare: sdk.DecCoins{
						sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
					},
				}, time.Now())
			},
			wantErr:     true,
			errContains: "delegated shares must be positive when validator is set",
		},
		{
			name: "non-zero delegated shares when not delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Shares: sdkmath.LegacyNewDec(100),
				}, time.Now())
			},
			wantErr:     true,
			errContains: "delegated shares must not be set when not delegated",
		},
		{
			name: "populated base rewards per share when not delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					BaseRewardsPerShare: sdk.DecCoins{
						sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
					},
				}, time.Now())
			},
			wantErr:     true,
			errContains: "base rewards per share must not be set when not delegated",
		},
		{
			name: "populated last bonus accrual when not delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{}, time.Now())
			},
			wantErr:     true,
			errContains: "last bonus accrual must not be set when not delegated",
		},
		{
			name: "exit_unlock_at set without exit_triggered_at",
			modify: func(p *types.Position) {
				p.ExitUnlockAt = time.Now().Add(time.Hour)
			},
			wantErr:     true,
			errContains: "exit_unlock_at must not be set for a position that is not exiting",
		},
		{
			name: "exit_unlock_at before exit_triggered_at",
			modify: func(p *types.Position) {
				now := time.Now()
				p.TriggerExit(now, -time.Hour)
			},
			wantErr:     true,
			errContains: "exit_unlock_at must be after exit_triggered_at",
		},
		{
			name: "exit_unlock_at equal to exit_triggered_at",
			modify: func(p *types.Position) {
				now := time.Now()
				p.TriggerExit(now, 0)
			},
			wantErr:     true,
			errContains: "exit_unlock_at must be after exit_triggered_at",
		},
		{
			name: "zero created_at_height",
			modify: func(p *types.Position) {
				p.CreatedAtHeight = 0
			},
			wantErr:     true,
			errContains: "created_at_height must be positive",
		},
		{
			name: "zero created_at_time",
			modify: func(p *types.Position) {
				p.CreatedAtTime = time.Time{}
			},
			wantErr:     true,
			errContains: "created_at_time must be non-zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pos := validPosition()
			tt.modify(&pos)
			err := pos.Validate()
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

func TestPosition_IsDelegated(t *testing.T) {
	t.Parallel()

	p := validPosition()
	require.True(t, p.IsDelegated())
}

func TestPosition_ClearExit(t *testing.T) {
	t.Parallel()

	pos := validPosition()
	now := time.Now()
	pos.TriggerExit(now, time.Hour*24)
	require.True(t, pos.HasTriggeredExit())

	pos.ClearExit()
	require.False(t, pos.HasTriggeredExit())
	require.NoError(t, pos.Validate())
}
