package types

import "fmt"

// DefaultGenesis returns the default inflation genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	if !gs.Params.DecayRate.IsPositive() && gs.DecayEpochStart != 0 {
		return fmt.Errorf("decay_epoch_start must be zero when decay is disabled")
	}

	return nil
}
