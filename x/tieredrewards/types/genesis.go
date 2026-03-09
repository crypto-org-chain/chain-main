package types

// DefaultGenesisState creates a default GenesisState object.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// ValidateGenesis validates the provided genesis state.
func ValidateGenesis(data GenesisState) error {
	return data.Params.Validate()
}
