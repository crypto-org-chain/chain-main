package types

import (
	"fmt"

	"gopkg.in/yaml.v2"

	sdkmath "cosmossdk.io/math"
)

// NewParams creates a new Params instance
func NewParams(maxSupply sdkmath.Int) Params {
	return Params{
		MaxSupply: maxSupply,
	}
}

// DefaultParams returns a default set of parameters, with max supply set to 0 meaning unlimited supply
func DefaultParams() Params {
	return NewParams(
		sdkmath.NewInt(0),
	)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateMaxSupply(p.MaxSupply); err != nil {
		return err
	}
	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// validateMaxSupply validates the MaxSupply param
func validateMaxSupply(v interface{}) error {
	maxSupply, ok := v.(sdkmath.Int)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if maxSupply.IsNil() {
		return fmt.Errorf("max supply cannot be nil")
	}

	if maxSupply.IsNegative() {
		return fmt.Errorf("max supply cannot be negative: %s", maxSupply)
	}

	return nil
}
