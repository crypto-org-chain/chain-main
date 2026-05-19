package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

var (
	testOwner     = sdk.AccAddress([]byte("test_owner__________")).String()
	testValidator = sdk.ValAddress([]byte("test_validator______")).String()
)

func validDelegatedPosition() types.Position {
	now := time.Now()
	return types.NewPosition(1, testOwner, 1, 100, 0, now, true, now)
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
			name:   "valid position",
			modify: func(*types.Position) {},
		},
		{
			name: "valid position with exit triggered",
			modify: func(p *types.Position) {
				p.TriggerExit(time.Now(), time.Hour*24*365)
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
			pos := validDelegatedPosition()
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
