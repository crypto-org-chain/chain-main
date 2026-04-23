package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	return types.NewPosition(id, testOwner, tierId, sdkmath.ZeroInt(), 100, types.Delegation{
		Validator:           testValidator,
		Shares:              sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{}, LastEventSeq: 0,
	}, time.Now())
}

func validFullGenesis() types.GenesisState {
	tier := genesisTier(1)
	pos := genesisPosition(1, 1)
	return types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos},
		NextPositionId: 2,
		ValidatorRewardRatios: []types.ValidatorRewardRatioEntry{
			{
				Validator:   testValidator,
				RewardRatio: types.ValidatorRewardRatio{},
			},
		},
		UnbondingDelegationMappings: []types.UnbondingMapping{
			{UnbondingId: 10, PositionId: 1},
		},
		RedelegationMappings: []types.UnbondingMapping{
			{UnbondingId: 11, PositionId: 1},
		},
		ValidatorPositionCounts: []types.ValidatorPositionCountEntry{
			{Validator: testValidator, Count: 1},
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

	t.Run("invalid validator address in reward ratio", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorRewardRatios[0].Validator = "invalid"
		require.ErrorContains(t, types.ValidateGenesis(genesis), "invalid validator address")
	})

	t.Run("duplicate validator in reward ratios", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorRewardRatios = append(genesis.ValidatorRewardRatios, genesis.ValidatorRewardRatios[0])
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate validator")
	})

	t.Run("malformed validator reward ratio cumulative rewards per share", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorRewardRatios[0].RewardRatio = types.ValidatorRewardRatio{
			CumulativeRewardsPerShare: sdk.DecCoins{
				{
					Denom:  "basecro",
					Amount: sdkmath.LegacyNewDec(-1),
				},
			},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "invalid reward ratio payload")
	})

	t.Run("unbonding mapping references unknown position", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.UnbondingDelegationMappings[0].PositionId = 999
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown position ID")
	})

	t.Run("duplicate unbonding IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.UnbondingDelegationMappings = append(genesis.UnbondingDelegationMappings, types.UnbondingMapping{UnbondingId: 10, PositionId: 1})
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate unbonding ID")
	})

	t.Run("redelegation mapping references unknown position", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings[0].PositionId = 999
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown position ID")
	})

	t.Run("duplicate redelegation IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings = append(genesis.RedelegationMappings, types.UnbondingMapping{UnbondingId: 11, PositionId: 1})
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate redelegation ID")
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
		genesis.ValidatorEventNextSeqs = []types.ValidatorEventNextSeqEntry{
			{Validator: testValidator, NextSeq: 3},
		}
		genesis.ValidatorPositionCounts = []types.ValidatorPositionCountEntry{
			{Validator: testValidator, Count: 1},
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("event sequence exceeds next_seq", func(t *testing.T) {
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
		genesis.ValidatorEventNextSeqs = []types.ValidatorEventNextSeqEntry{
			{Validator: testValidator, NextSeq: 2},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "next_seq (2) must be greater than max event sequence (3)")
	})

	t.Run("events without next_seq entry", func(t *testing.T) {
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
		// No ValidatorEventNextSeqs
		genesis.ValidatorEventNextSeqs = nil
		require.ErrorContains(t, types.ValidateGenesis(genesis), "has events but no next_seq entry")
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
		genesis.ValidatorEventNextSeqs = []types.ValidatorEventNextSeqEntry{
			{Validator: testValidator, NextSeq: 2},
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
		genesis.ValidatorEventNextSeqs = []types.ValidatorEventNextSeqEntry{
			{Validator: testValidator, NextSeq: 2},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate validator event")
	})

	t.Run("delegated position LastEventSeq exceeds validator latest event seq", func(t *testing.T) {
		genesis := validFullGenesis()
		// Position is delegated to testValidator. Set LastEventSeq = 5.
		genesis.Positions[0].LastEventSeq = 5
		// Validator has one event at seq 1.
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
		genesis.ValidatorEventNextSeqs = []types.ValidatorEventNextSeqEntry{
			{Validator: testValidator, NextSeq: 2},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "LastEventSeq (5) greater than validator")
	})

	t.Run("delegated position LastEventSeq exceeds when no events exist", func(t *testing.T) {
		genesis := validFullGenesis()
		// Position is delegated to testValidator. Set LastEventSeq = 1.
		// But no events exist for the validator.
		genesis.Positions[0].LastEventSeq = 1
		require.ErrorContains(t, types.ValidateGenesis(genesis), "LastEventSeq (1) greater than validator")
	})

	t.Run("delegated position LastEventSeq at latest event seq is valid", func(t *testing.T) {
		genesis := validFullGenesis()
		// Position LastEventSeq = 1, validator has event at seq 1.
		genesis.Positions[0].LastEventSeq = 1
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
		genesis.ValidatorEventNextSeqs = []types.ValidatorEventNextSeqEntry{
			{Validator: testValidator, NextSeq: 2},
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("position count mismatch — count too high", func(t *testing.T) {
		genesis := validFullGenesis()
		// 1 delegated position on testValidator, but count says 5.
		genesis.ValidatorPositionCounts = []types.ValidatorPositionCountEntry{
			{Validator: testValidator, Count: 5},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "position count mismatch")
	})

	t.Run("position count mismatch — count too low", func(t *testing.T) {
		genesis := validFullGenesis()
		// 1 delegated position on testValidator, but count says 0 (missing entry).
		// No ValidatorPositionCounts entry → importedCounts[testValidator] = 0, expected = 1.
		genesis.ValidatorPositionCounts = nil
		require.ErrorContains(t, types.ValidateGenesis(genesis), "position count mismatch")
	})

	t.Run("position count for validator with no positions", func(t *testing.T) {
		genesis := validFullGenesis()
		otherVal := sdk.ValAddress([]byte("other_validator_____")).String()
		genesis.ValidatorPositionCounts = []types.ValidatorPositionCountEntry{
			{Validator: testValidator, Count: 1},
			{Validator: otherVal, Count: 3},
		}
		require.ErrorContains(t, types.ValidateGenesis(genesis), "no delegated positions")
	})
}
