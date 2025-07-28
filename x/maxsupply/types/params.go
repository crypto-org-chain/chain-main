package types

import (
	"fmt"

	"gopkg.in/yaml.v2"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewParams creates a new Params instance
func NewParams(maxSupply sdkmath.Int, burnedAddresses []string) Params {
	return Params{
		MaxSupply:       maxSupply,
		BurnedAddresses: burnedAddresses,
	}
}

// DefaultParams returns a default set of parameters, with max supply set to 0 meaning unlimited supply
func DefaultParams() Params {
	return NewParams(
		sdkmath.NewInt(0),
		[]string{},
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

// validateBurnedAddresses validates the BurnedAddresses param
func validateBurnedAddresses(v interface{}) error {
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

		if addr == "" {
			return fmt.Errorf("burned address cannot be empty at index %d", i)
		}
	}

	return nil
}
