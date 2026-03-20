package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesisState creates a default GenesisState object.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// ValidateGenesis validates the provided genesis state.
func ValidateGenesis(data GenesisState) error {
	if err := data.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Validate tiers and collect IDs for cross-referencing.
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

	// Validate positions and collect IDs.
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

		// Cross-reference: position must reference an existing tier.
		if _, ok := tierIDs[pos.TierId]; !ok {
			return fmt.Errorf("position %d references unknown tier ID %d", pos.Id, pos.TierId)
		}

		if pos.Id > maxPosID {
			maxPosID = pos.Id
		}
	}

	// NextPositionId must be >= highest position ID + 1.
	if len(data.Positions) > 0 && data.NextPositionId <= maxPosID {
		return fmt.Errorf("next_position_id (%d) must be greater than the highest position ID (%d)", data.NextPositionId, maxPosID)
	}

	// Validate validator reward ratio entries.
	seenValidators := make(map[string]struct{}, len(data.ValidatorRewardRatios))
	for i, entry := range data.ValidatorRewardRatios {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in reward ratio at index %d: %w", i, err)
		}
		if _, dup := seenValidators[entry.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in reward ratios at index %d", entry.Validator, i)
		}
		seenValidators[entry.Validator] = struct{}{}
	}

	// Validate unbonding mappings.
	seenUnbondingIDs := make(map[uint64]struct{}, len(data.UnbondingMappings))
	for i, mapping := range data.UnbondingMappings {
		if _, dup := seenUnbondingIDs[mapping.UnbondingId]; dup {
			return fmt.Errorf("duplicate unbonding ID %d at index %d", mapping.UnbondingId, i)
		}
		seenUnbondingIDs[mapping.UnbondingId] = struct{}{}

		// Cross-reference: unbonding mapping must reference an existing position.
		if _, ok := posIDs[mapping.PositionId]; !ok {
			return fmt.Errorf("unbonding mapping at index %d references unknown position ID %d", i, mapping.PositionId)
		}
	}

	return nil
}
