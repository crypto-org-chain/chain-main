package types

import "fmt"

// DefaultGenesisState creates a default GenesisState object.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:         DefaultParams(),
		Positions:      []TierPosition{},
		NextPositionId: 1,
	}
}

// ValidateGenesis validates the provided genesis state.
func ValidateGenesis(data GenesisState) error {
	if err := data.Params.Validate(); err != nil {
		return err
	}

	// Validate positions.
	seenIDs := make(map[uint64]bool)
	var maxPositionId uint64
	for _, pos := range data.Positions {
		if seenIDs[pos.PositionId] {
			return fmt.Errorf("duplicate position ID: %d", pos.PositionId)
		}
		seenIDs[pos.PositionId] = true

		if pos.Owner == "" {
			return fmt.Errorf("position %d: owner cannot be empty", pos.PositionId)
		}
		if pos.AmountLocked.IsNegative() {
			return fmt.Errorf("position %d: amount_locked cannot be negative", pos.PositionId)
		}
		if pos.PositionId > maxPositionId {
			maxPositionId = pos.PositionId
		}
	}

	// NextPositionId must be greater than all existing position IDs.
	if len(data.Positions) > 0 && data.NextPositionId <= maxPositionId {
		return fmt.Errorf("next_position_id %d must be greater than max position ID %d", data.NextPositionId, maxPositionId)
	}

	return nil
}
