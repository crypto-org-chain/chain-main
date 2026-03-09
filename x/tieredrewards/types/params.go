package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams(targetBaseRewardRate sdkmath.LegacyDec) Params {
	return Params{
		TargetBaseRewardRate: targetBaseRewardRate,
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
	return validateTargetBaseRewardRate(p.TargetBaseRewardRate)
}

func validateTargetBaseRewardRate(v sdkmath.LegacyDec) error {
	if v.IsNil() {
		return fmt.Errorf("target base reward rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("target base reward rate cannot be negative: %s", v)
	}

	return nil
}
