package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func genesisTier(id uint32) types.Tier {
	return types.Tier{
		Id:            id,
		ExitDuration:  time.Hour * 24 * 365,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(1000),
	}
}

func genesisPosition(id uint64, tierId uint32) types.Position {
	now := time.Now()
	return types.NewPosition(id, testOwner, tierId, 100, 0, now, true, now)
}

func validFullGenesis() types.GenesisState {
	tier := genesisTier(1)
	pos := genesisPosition(1, 1)
	return types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos},
		NextPositionId: 2,
		RedelegationMappings: []types.RedelegationMapping{
			{UnbondingId: 1, PositionId: 1},
		},
	}
}

func TestValidateGenesis(t *testing.T) {
	t.Run("valid default genesis", func(t *testing.T) {
		genesis := types.DefaultGenesisState()
		require.NoError(t, types.ValidateGenesis(*genesis))
	})

	t.Run("valid custom genesis", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.NewParams(sdkmath.LegacyNewDecWithPrec(3, 2)),
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("invalid genesis - negative rate", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.NewParams(sdkmath.LegacyNewDec(-1)),
		}
		require.Error(t, types.ValidateGenesis(genesis))
	})

	t.Run("valid full genesis", func(t *testing.T) {
		genesis := validFullGenesis()
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("duplicate tier IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Tiers = append(genesis.Tiers, genesisTier(1))
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate tier ID")
	})

	t.Run("invalid tier fields", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Tiers[0].ExitDuration = 0 // invalid
		require.ErrorContains(t, types.ValidateGenesis(genesis), "invalid tier")
	})

	t.Run("duplicate position IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Positions = append(genesis.Positions, genesisPosition(1, 1))
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate position ID")
	})

	t.Run("position references unknown tier", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Positions[0].TierId = 99
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown tier ID")
	})

	t.Run("NextPositionId too low", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.NextPositionId = 1 // must be > position ID 1
		require.ErrorContains(t, types.ValidateGenesis(genesis), "next_position_id")
	})

	t.Run("redelegation mapping references unknown position", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings[0].PositionId = 999
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown position ID")
	})

	t.Run("duplicate redelegation mapping unbonding id", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings = append(genesis.RedelegationMappings, types.RedelegationMapping{
			UnbondingId: 1,
			PositionId:  1,
		})
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate redelegation mapping unbonding_id")
	})

	t.Run("redelegation mapping zero unbonding id", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings[0].UnbondingId = 0
		require.ErrorContains(t, types.ValidateGenesis(genesis), "zero unbonding_id")
	})

	t.Run("valid genesis with events", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorEvents = []types.ValidatorEventEntry{
			{
				Validator: testValidator,
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         100,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 1,
				},
			},
			{
				Validator: testValidator,
				Sequence:  2,
				Event: types.ValidatorEvent{
					Height:         101,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_UNBOND,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 1,
				},
			},
		}
		genesis.ValidatorEventSeqs = []types.ValidatorEventSeqEntry{
			{Validator: testValidator, CurrentSeq: 2},
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("event sequence exceeds current_seq", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorEvents = []types.ValidatorEventEntry{
			{
				Validator: testValidator,
				Sequence:  3,
				Event: types.ValidatorEvent{
					Height:         100,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 1,
				},
			},
		}
		genesis.ValidatorEventSeqs = []types.ValidatorEventSeqEntry{
			{Validator: testValidator, CurrentSeq: 2},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "current_seq (2) must be greater than or equal to max event sequence (3)")
	})

	t.Run("events without current_seq entry", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorEvents = []types.ValidatorEventEntry{
			{
				Validator: testValidator,
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         100,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 1,
				},
			},
		}
		// No ValidatorEventSeqs
		genesis.ValidatorEventSeqs = nil
		require.ErrorContains(t, types.ValidateGenesis(genesis), "has events but no current_seq entry")
	})

	t.Run("zero reference count in event", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorEvents = []types.ValidatorEventEntry{
			{
				Validator: testValidator,
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         100,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 0,
				},
			},
		}
		genesis.ValidatorEventSeqs = []types.ValidatorEventSeqEntry{
			{Validator: testValidator, CurrentSeq: 1},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "zero reference count")
	})

	t.Run("duplicate validator event", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorEvents = []types.ValidatorEventEntry{
			{
				Validator: testValidator,
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         100,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 1,
				},
			},
			{
				Validator: testValidator,
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         101,
					Timestamp:      time.Now(),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_UNBOND,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 1,
				},
			},
		}
		genesis.ValidatorEventSeqs = []types.ValidatorEventSeqEntry{
			{Validator: testValidator, CurrentSeq: 1},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate validator event")
	})

	t.Run("delegated position processed all events — no events remain", func(t *testing.T) {
		genesis := validFullGenesis()
		// Position LastEventSeq = 1 (processed event 1), no events remain (GC'd).
		genesis.Positions[0].LastEventSeq = 1
		genesis.ValidatorEventSeqs = []types.ValidatorEventSeqEntry{
			{Validator: testValidator, CurrentSeq: 1},
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})
}
