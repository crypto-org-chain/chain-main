package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	testOwnerAddr = sdk.AccAddress([]byte("test_owner__________"))
	testOwner     = testOwnerAddr.String()
	testValidator = sdk.ValAddress([]byte("test_validator______")).String()
)

func validDelegatedPosition() types.Position {
	now := time.Now()
	pos := types.NewPosition(1, testOwner, 1, testutil.DelegatorAddress(testOwnerAddr, 1), 100, 0, now, true, now)
	return pos
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
			name: "empty delegator address",
			modify: func(p *types.Position) {
				p.DelegatorAddress = ""
			},
			wantErr:     true,
			errContains: "invalid delegator address",
		},
		{
			name: "invalid delegator address",
			modify: func(p *types.Position) {
				p.DelegatorAddress = "not_a_valid_address"
			},
			wantErr:     true,
			errContains: "invalid delegator address",
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

// TestDerivePositionDelegatorAddress_Deterministic asserts the v2 derivation
// is a pure function of its inputs.
func TestDerivePositionDelegatorAddress_Deterministic(t *testing.T) {
	t.Parallel()

	headerHash := []byte{0xab, 0xcd, 0xef, 0x01, 0x02, 0x03}
	owner := sdk.AccAddress([]byte("owner_______________"))

	first := types.DerivePositionDelegatorAddress(headerHash, owner, 7, 0)
	second := types.DerivePositionDelegatorAddress(headerHash, owner, 7, 0)
	require.Equal(t, first, second, "derivation must be deterministic for fixed inputs")
}

// TestDerivePositionDelegatorAddress_DiffersByHeaderHash confirms that two
// different block hashes produce different addresses for the same (owner, id, nonce).
// This is the property that makes pre-block poisoning infeasible.
func TestDerivePositionDelegatorAddress_DiffersByHeaderHash(t *testing.T) {
	t.Parallel()

	owner := sdk.AccAddress([]byte("owner_______________"))
	a := types.DerivePositionDelegatorAddress([]byte{0x01}, owner, 1, 0)
	b := types.DerivePositionDelegatorAddress([]byte{0x02}, owner, 1, 0)
	require.NotEqual(t, a, b, "different headerHash values must yield different addresses")
}

// TestDerivePositionDelegatorAddress_DiffersByOwner asserts that the same
// (headerHash, id, nonce) for two different owners produces different addresses.
func TestDerivePositionDelegatorAddress_DiffersByOwner(t *testing.T) {
	t.Parallel()

	headerHash := []byte{0xff}
	ownerA := sdk.AccAddress([]byte("owner_a_____________"))
	ownerB := sdk.AccAddress([]byte("owner_b_____________"))
	a := types.DerivePositionDelegatorAddress(headerHash, ownerA, 1, 0)
	b := types.DerivePositionDelegatorAddress(headerHash, ownerB, 1, 0)
	require.NotEqual(t, a, b, "different owners must yield different addresses")
}

// TestDerivePositionDelegatorAddress_DiffersById asserts uniqueness across ids
// for fixed (headerHash, owner, nonce).
func TestDerivePositionDelegatorAddress_DiffersById(t *testing.T) {
	t.Parallel()

	headerHash := []byte{0xff}
	owner := sdk.AccAddress([]byte("owner_______________"))
	a := types.DerivePositionDelegatorAddress(headerHash, owner, 1, 0)
	b := types.DerivePositionDelegatorAddress(headerHash, owner, 2, 0)
	require.NotEqual(t, a, b, "different ids must yield different addresses")
}

// TestDerivePositionDelegatorAddress_DiffersByNonce confirms that incrementing
// the collision nonce yields a fresh address — required for the keeper's
// collision-handling loop to make progress on the (improbable) chance of
// preimage collision.
func TestDerivePositionDelegatorAddress_DiffersByNonce(t *testing.T) {
	t.Parallel()

	headerHash := []byte{0xff}
	owner := sdk.AccAddress([]byte("owner_______________"))
	a := types.DerivePositionDelegatorAddress(headerHash, owner, 1, 0)
	b := types.DerivePositionDelegatorAddress(headerHash, owner, 1, 1)
	require.NotEqual(t, a, b, "different nonces must yield different addresses")
}
