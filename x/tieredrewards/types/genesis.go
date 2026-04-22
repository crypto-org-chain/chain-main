package types

import (
	"fmt"

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

	seenValidators := make(map[string]struct{}, len(data.ValidatorRewardRatios))
	for i, entry := range data.ValidatorRewardRatios {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in reward ratio at index %d: %w", i, err)
		}
		if err := entry.RewardRatio.CumulativeRewardsPerShare.Validate(); err != nil {
			return fmt.Errorf("invalid reward ratio payload at index %d: %w", i, err)
		}
		if _, dup := seenValidators[entry.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in reward ratios at index %d", entry.Validator, i)
		}
		seenValidators[entry.Validator] = struct{}{}
	}

	seenUnbondingIDs := make(map[uint64]struct{}, len(data.UnbondingDelegationMappings))
	for i, mapping := range data.UnbondingDelegationMappings {
		if _, dup := seenUnbondingIDs[mapping.UnbondingId]; dup {
			return fmt.Errorf("duplicate unbonding ID %d at index %d", mapping.UnbondingId, i)
		}
		seenUnbondingIDs[mapping.UnbondingId] = struct{}{}

		if _, ok := posIDs[mapping.PositionId]; !ok {
			return fmt.Errorf("unbonding mapping at index %d references unknown position ID %d", i, mapping.PositionId)
		}
	}
	// redelegation unbonding ids share the same global counter as unbonding delegation ids, so there should be no duplicates.
	for i, mapping := range data.RedelegationMappings {
		if _, dup := seenUnbondingIDs[mapping.UnbondingId]; dup {
			return fmt.Errorf("duplicate redelegation ID %d at index %d", mapping.UnbondingId, i)
		}
		seenUnbondingIDs[mapping.UnbondingId] = struct{}{}

		if _, ok := posIDs[mapping.PositionId]; !ok {
			return fmt.Errorf("redelegation mapping at index %d references unknown position ID %d", i, mapping.PositionId)
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

	// Validate event next sequences.
	seenNextSeq := make(map[string]struct{}, len(data.ValidatorEventNextSeqs))
	for i, entry := range data.ValidatorEventNextSeqs {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in event next seq at index %d: %w", i, err)
		}
		if _, dup := seenNextSeq[entry.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in event next seqs at index %d", entry.Validator, i)
		}
		seenNextSeq[entry.Validator] = struct{}{}
	}

	// Validate position counts.
	seenCountValidators := make(map[string]struct{}, len(data.ValidatorPositionCounts))
	for i, entry := range data.ValidatorPositionCounts {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in position count at index %d: %w", i, err)
		}
		if _, dup := seenCountValidators[entry.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in position counts at index %d", entry.Validator, i)
		}
		seenCountValidators[entry.Validator] = struct{}{}
	}

	// Cross-validate: next_seq must be consistent with event sequences.
	// Build max sequence per validator from events.
	maxSeqByValidator := make(map[string]uint64)
	for _, entry := range data.ValidatorEvents {
		if entry.Sequence > maxSeqByValidator[entry.Validator] {
			maxSeqByValidator[entry.Validator] = entry.Sequence
		}
	}

	// Cross-validate: next_seq must exceed max event sequence.
	nextSeqByValidator := make(map[string]uint64)
	for _, entry := range data.ValidatorEventNextSeqs {
		nextSeqByValidator[entry.Validator] = entry.NextSeq
	}
	for val, maxSeq := range maxSeqByValidator {
		nextSeq, ok := nextSeqByValidator[val]
		if !ok {
			return fmt.Errorf("validator %s has events but no next_seq entry", val)
		}
		if nextSeq <= maxSeq {
			return fmt.Errorf("validator %s next_seq (%d) must be greater than max event sequence (%d)", val, nextSeq, maxSeq)
		}
	}

	return nil
}
