package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestPositionState_IsDelegated(t *testing.T) {
	t.Run("nil delegation", func(t *testing.T) {
		state := types.PositionState{Position: validPosition()}
		require.False(t, state.IsDelegated())
	})

	t.Run("non-nil delegation", func(t *testing.T) {
		state := types.PositionState{
			Position: validPosition(),
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
	pos := validPosition()
	state := types.PositionState{Position: pos}

	require.Equal(t, pos.Id, state.Id)
	require.Equal(t, pos.Owner, state.Owner)
	require.Equal(t, pos.TierId, state.TierId)
}
