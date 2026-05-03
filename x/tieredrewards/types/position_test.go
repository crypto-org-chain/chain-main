package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

var (
	testOwner     = sdk.AccAddress([]byte("test_owner__________")).String()
	testValidator = sdk.ValAddress([]byte("test_validator______")).String()
)

func validPosition() types.Position {
	return types.NewPosition(1, testOwner, 1, sdkmath.ZeroInt(), 100, types.Delegation{
		Validator:    testValidator,
		Shares:       sdkmath.LegacyNewDec(1000),
		LastEventSeq: 0,
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
					Validator:    testValidator,
					Shares:       sdkmath.LegacyNewDec(1000),
					LastEventSeq: 0,
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
			name: "valid undelegated position - non-zero amount without exit (e.g. after redeleg slash + AddToTier)",
			modify: func(p *types.Position) {
				p.ClearDelegation()
				p.UpdateAmount(sdkmath.NewInt(1000))
			},
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
			name: "nil amount",
			modify: func(p *types.Position) {
				p.UpdateAmount(sdkmath.Int{})
			},
			wantErr:     true,
			errContains: "amount cannot be nil",
		},
		{
			name: "negative amount",
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
					Validator:    testValidator,
					Shares:       sdkmath.LegacyNewDec(-1),
					LastEventSeq: 0,
				}, time.Now())
			},
			wantErr:     true,
			errContains: "delegated shares must be positive when validator is set",
		},
		{
			name: "invalid validator address when delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Validator:    "not_valid",
					Shares:       sdkmath.LegacyNewDec(100),
					LastEventSeq: 0,
				}, time.Now())
			},
			wantErr:     true,
			errContains: "invalid validator address",
		},
		{
			name: "zero delegated shares when delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Validator:    testValidator,
					Shares:       sdkmath.LegacyZeroDec(),
					LastEventSeq: 0,
				}, time.Now())
			},
			wantErr:     true,
			errContains: "delegated shares must be positive when validator is set",
		},
		{
			name: "non-zero delegated shares when not delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{
					Shares:       sdkmath.LegacyNewDec(100),
					LastEventSeq: 0,
				}, time.Now())
			},
			wantErr:     true,
			errContains: "delegated shares must not be set when not delegated",
		},
		{
			name: "populated last bonus accrual when not delegated",
			modify: func(p *types.Position) {
				p.WithDelegation(types.Delegation{LastEventSeq: 0}, time.Now())
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
		{
			name: "non zero last event seq when not delegated",
			modify: func(p *types.Position) {
				p.ClearDelegation()
				p.LastEventSeq = 1
			},
			wantErr:     true,
			errContains: "last event seq must not be set when not delegated",
		},
		{
			name: "last known bonded true when not delegated",
			modify: func(p *types.Position) {
				p.ClearDelegation()
				p.LastKnownBonded = true
			},
			wantErr:     true,
			errContains: "last known bonded must not be true when not delegated",
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

	pos.ClearExit(now)
	require.False(t, pos.HasTriggeredExit())
	require.NoError(t, pos.Validate())
}

// TestGetDelegatorAddress_Deterministic verifies that GetDelegatorAddress
// is a pure function of the position id — repeated calls produce the same
// address.
func TestGetDelegatorAddress_Deterministic(t *testing.T) {
	t.Parallel()

	for _, id := range []uint64{0, 1, 42, 1_000_000, 1 << 63} {
		first := types.GetDelegatorAddress(id)
		second := types.GetDelegatorAddress(id)
		require.Equal(t, first, second, "GetDelegatorAddress must be deterministic for position id %d", id)
		require.Len(t, first, 20, "derived address must be 20 bytes")
	}
}

// TestGetDelegatorAddress_UniquePerID verifies that distinct position ids
// produce distinct delegator addresses.
func TestGetDelegatorAddress_UniquePerID(t *testing.T) {
	t.Parallel()

	const n = 1000
	seen := make(map[string]uint64, n)
	for i := uint64(0); i < n; i++ {
		addr := types.GetDelegatorAddress(i)
		key := string(addr)
		if prev, dup := seen[key]; dup {
			t.Fatalf("collision: position %d and %d derived to the same delegator address %s", prev, i, addr.String())
		}
		seen[key] = i
	}
	require.Len(t, seen, n, "expected %d unique delegator addresses", n)
}

// TestGetDelegatorAddress_DistinctFromModuleAccount verifies that a position's
// delegator address never collides with the tieredrewards module account or
// the rewards-pool account.
func TestGetDelegatorAddress_DistinctFromModuleAccount(t *testing.T) {
	t.Parallel()

	poolAddr := authtypes.NewModuleAddress(types.RewardsPoolName)

	for _, id := range []uint64{0, 1, 42} {
		delAddr := types.GetDelegatorAddress(id)
		require.False(t, delAddr.Equals(poolAddr),
			"position %d delegator address must differ from the rewards pool address", id)
	}
}
