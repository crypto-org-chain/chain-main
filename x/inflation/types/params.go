package types

import (
	"fmt"

	"gopkg.in/yaml.v2"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	UNLIMITED_SUPPLY = 0
)

// NewParams creates a new Params instance
func NewParams(maxSupply sdkmath.Int, burnedAddresses []string, decayRate sdkmath.LegacyDec) Params {
	return Params{
		MaxSupply:       maxSupply,
		BurnedAddresses: burnedAddresses,
		DecayRate:       decayRate,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(
		sdkmath.NewInt(UNLIMITED_SUPPLY),
		[]string{},
		sdkmath.LegacyZeroDec(), // default to 0 (decay disabled)
	)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateMaxSupply(p.MaxSupply); err != nil {
		return err
	}

	if err := validateBurnedAddresses(p.BurnedAddresses); err != nil {
		return err
	}

	if err := validateDecayRate(p.DecayRate); err != nil {
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
func validateMaxSupply(v any) error {
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

// validateBurnedAddresses validates the BurnedAddresses param
func validateBurnedAddresses(v any) error {
	burnedAddresses, ok := v.([]string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	addressMap := make(map[string]bool)

	for i, addr := range burnedAddresses {
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return fmt.Errorf("invalid burned address at index %d: %s, error: %w", i, addr, err)
		}

		if addressMap[addr] {
			return fmt.Errorf("duplicate burned address found: %s", addr)
		}
		addressMap[addr] = true

	}

	return nil
}

func validateDecayRate(i any) error {
	v, ok := i.(sdkmath.LegacyDec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("decay rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("decay rate cannot be negative (must be between 0 and 1 inclusive, got: %s)", v)
	}

	if v.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("decay rate too large (must be between 0 and 1 inclusive, got: %s)", v)
	}

	return nil
}
