package types

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

func ValidateGenesis(data GenesisState) error {
	if err := data.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	tierIDs := make(map[uint32]struct{}, len(data.Tiers))
	for i, tier := range data.Tiers {
		if err := tier.Validate(); err != nil {
			return fmt.Errorf("invalid tier at index %d: %w", i, err)
		}
		if _, dup := tierIDs[tier.Id]; dup {
			return fmt.Errorf("duplicate tier ID %d at index %d", tier.Id, i)
		}
		tierIDs[tier.Id] = struct{}{}
	}

	posIDs := make(map[uint64]struct{}, len(data.Positions))
	var maxPosID uint64
	for i, pos := range data.Positions {
		if err := pos.Validate(); err != nil {
			return fmt.Errorf("invalid position at index %d: %w", i, err)
		}
		if _, dup := posIDs[pos.Id]; dup {
			return fmt.Errorf("duplicate position ID %d at index %d", pos.Id, i)
		}
		posIDs[pos.Id] = struct{}{}

		if _, ok := tierIDs[pos.TierId]; !ok {
			return fmt.Errorf("position %d references unknown tier ID %d", pos.Id, pos.TierId)
		}

		if pos.Id > maxPosID {
			maxPosID = pos.Id
		}
	}

	if len(data.Positions) > 0 && data.NextPositionId <= maxPosID {
		return fmt.Errorf("next_position_id (%d) must be greater than the highest position ID (%d)", data.NextPositionId, maxPosID)
	}

	seenUnbondingIds := make(map[uint64]struct{}, len(data.RedelegationMappings))
	for i, entry := range data.RedelegationMappings {
		if entry.UnbondingId == 0 {
			return fmt.Errorf("redelegation mapping at index %d has zero unbonding_id", i)
		}
		if _, dup := seenUnbondingIds[entry.UnbondingId]; dup {
			return fmt.Errorf("duplicate redelegation mapping unbonding_id %d at index %d", entry.UnbondingId, i)
		}
		seenUnbondingIds[entry.UnbondingId] = struct{}{}

		if _, ok := posIDs[entry.PositionId]; !ok {
			return fmt.Errorf("redelegation mapping at index %d references unknown position ID %d", i, entry.PositionId)
		}
	}

	// Validate validator events.
	type eventKey struct {
		validator string
		seq       uint64
	}
	seenEvents := make(map[eventKey]struct{}, len(data.ValidatorEvents))
	for i, entry := range data.ValidatorEvents {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in event at index %d: %w", i, err)
		}
		key := eventKey{validator: entry.Validator, seq: entry.Sequence}
		if _, dup := seenEvents[key]; dup {
			return fmt.Errorf("duplicate validator event (validator=%s, seq=%d) at index %d", entry.Validator, entry.Sequence, i)
		}
		seenEvents[key] = struct{}{}
		if entry.Event.ReferenceCount == 0 {
			return fmt.Errorf("validator event at index %d has zero reference count", i)
		}
	}

	// Validate event current sequences.
	seenCurrentSeq := make(map[string]struct{}, len(data.ValidatorEventSeqs))
	for i, entry := range data.ValidatorEventSeqs {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in event current seq at index %d: %w", i, err)
		}
		if _, dup := seenCurrentSeq[entry.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in event current seqs at index %d", entry.Validator, i)
		}
		seenCurrentSeq[entry.Validator] = struct{}{}
	}

	// Cross-validate: current_seq must be consistent with event sequences.
	// Build max sequence per validator from events.
	maxSeqByValidator := make(map[string]uint64)
	for _, entry := range data.ValidatorEvents {
		if entry.Sequence > maxSeqByValidator[entry.Validator] {
			maxSeqByValidator[entry.Validator] = entry.Sequence
		}
	}

	// Cross-validate: current_seq must be >= max event sequence.
	// current_seq is the last used seq, so it must be at least as large as the
	// highest event seq (could be larger if events were garbage-collected).
	currentSeqByValidator := make(map[string]uint64)
	for _, entry := range data.ValidatorEventSeqs {
		currentSeqByValidator[entry.Validator] = entry.CurrentSeq
	}
	sortedVals := make([]string, 0, len(maxSeqByValidator))
	for val := range maxSeqByValidator {
		sortedVals = append(sortedVals, val)
	}
	sort.Strings(sortedVals)
	for _, val := range sortedVals {
		maxSeq := maxSeqByValidator[val]
		currentSeq, ok := currentSeqByValidator[val]
		if !ok {
			return fmt.Errorf("validator %s has events but no current_seq entry", val)
		}
		if currentSeq < maxSeq {
			return fmt.Errorf("validator %s current_seq (%d) must be greater than or equal to max event sequence (%d)", val, currentSeq, maxSeq)
		}
	}

	return nil
}
