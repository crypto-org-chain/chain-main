package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func validDelegation() *stakingtypes.Delegation {
	return &stakingtypes.Delegation{
		DelegatorAddress: types.GetDelegatorAddress(1).String(),
		ValidatorAddress: testValidator,
		Shares:           sdkmath.LegacyNewDec(1000),
	}
}

// TestPositionState_Validate covers the delegation-vs-bonus-state invariants
// that used to live on Position.Validate and now live on PositionState.Validate.
func TestPositionState_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		build       func() types.PositionState
		wantErr     bool
		errContains string
	}{
		{
			name: "valid delegated",
			build: func() types.PositionState {
				return types.PositionState{Position: validDelegatedPosition(), Delegation: validDelegation()}
			},
		},
		{
			name: "valid undelegated",
			build: func() types.PositionState {
				pos := validDelegatedPosition()
				pos.ResetBonusCheckpoints()
				return types.PositionState{Position: pos}
			},
		},
		{
			name: "delegated but LastKnownBonded false (e.g. after unbond event)",
			build: func() types.PositionState {
				pos := validDelegatedPosition()
				pos.LastKnownBonded = false
				return types.PositionState{Position: pos, Delegation: validDelegation()}
			},
		},
		{
			name: "delegated but LastBonusAccrual zero",
			build: func() types.PositionState {
				pos := validDelegatedPosition()
				pos.LastBonusAccrual = time.Time{}
				return types.PositionState{Position: pos, Delegation: validDelegation()}
			},
			wantErr:     true,
			errContains: "last_bonus_accrual must be non-zero when position is delegated",
		},
		{
			name: "undelegated but LastBonusAccrual non-zero",
			build: func() types.PositionState {
				pos := validDelegatedPosition()
				pos.ResetBonusCheckpoints()
				pos.LastBonusAccrual = time.Now()
				return types.PositionState{Position: pos}
			},
			wantErr:     true,
			errContains: "last_bonus_accrual must be zero when position is undelegated",
		},
		{
			name: "undelegated but LastEventSeq non-zero",
			build: func() types.PositionState {
				pos := validDelegatedPosition()
				pos.ResetBonusCheckpoints()
				pos.LastEventSeq = 5
				return types.PositionState{Position: pos}
			},
			wantErr:     true,
			errContains: "last_event_seq must be zero when position is undelegated",
		},
		{
			name: "undelegated but LastKnownBonded true",
			build: func() types.PositionState {
				pos := validDelegatedPosition()
				pos.ResetBonusCheckpoints()
				pos.LastKnownBonded = true
				return types.PositionState{Position: pos}
			},
			wantErr:     true,
			errContains: "last_known_bonded must be false when position is undelegated",
		},
		{
			name: "delegation.DelegatorAddress does not match GetDelegatorAddress(pos.Id)",
			build: func() types.PositionState {
				del := validDelegation()
				del.DelegatorAddress = types.GetDelegatorAddress(999).String()
				return types.PositionState{Position: validDelegatedPosition(), Delegation: del}
			},
			wantErr:     true,
			errContains: "does not match expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			state := tt.build()
			err := state.Validate()
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

func TestPositionState_IsDelegated(t *testing.T) {
	t.Run("nil delegation", func(t *testing.T) {
		state := types.PositionState{Position: validDelegatedPosition()}
		require.False(t, state.IsDelegated())
	})

	t.Run("non-nil delegation", func(t *testing.T) {
		state := types.PositionState{
			Position: validDelegatedPosition(),
			Delegation: &stakingtypes.Delegation{
				DelegatorAddress: "delegator",
				ValidatorAddress: "validator",
				Shares:           sdkmath.LegacyNewDec(1000),
			},
		}
		require.True(t, state.IsDelegated())
	})
}

func TestPositionState_PromotesEmbeddedFields(t *testing.T) {
	pos := validDelegatedPosition()
	state := types.PositionState{Position: pos}

	require.Equal(t, pos.Id, state.Id)
	require.Equal(t, pos.Owner, state.Owner)
	require.Equal(t, pos.TierId, state.TierId)
}

func TestPosition_ClearExit_Delegated(t *testing.T) {
	t.Parallel()

	pos := validDelegatedPosition()
	now := time.Now()
	pos.TriggerExit(now, time.Hour*24)
	require.True(t, pos.HasTriggeredExit())

	posState := types.PositionState{Position: pos, Delegation: validDelegation()}
	posState.ClearExit(now)
	require.False(t, posState.HasTriggeredExit())
	require.True(t, posState.LastBonusAccrual.Equal(now))
	require.NoError(t, posState.Validate())
}

func TestPosition_ClearExit_Undelegated(t *testing.T) {
	t.Parallel()

	pos := validDelegatedPosition()
	pos.ResetBonusCheckpoints()
	now := time.Now()
	pos.TriggerExit(now, time.Hour*24)
	require.True(t, pos.HasTriggeredExit())

	posState := types.PositionState{Position: pos}
	posState.ClearExit(now)
	require.False(t, posState.HasTriggeredExit())
	require.True(t, posState.LastBonusAccrual.IsZero())
	require.NoError(t, posState.Validate())
}
