package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams(targetBaseRewardsRate sdkmath.LegacyDec) Params {
	return Params{
		TargetBaseRewardsRate: targetBaseRewardsRate,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		sdkmath.LegacyZeroDec(),
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	return validateTargetBaseRewardsRate(p.TargetBaseRewardsRate)
}

func validateTargetBaseRewardsRate(v sdkmath.LegacyDec) error {
	if v.IsNil() {
		return fmt.Errorf("target base rewards rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("target base rewards rate cannot be negative: %s", v)
	}

	// Cap at 100% to prevent governance from draining the rewards pool via BeginBlock top-ups.
	if v.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("target base rewards rate must not exceed 1.0 (100%%): got %s", v)
	}

	return nil
}
