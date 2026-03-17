package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewParams creates a new Params instance.
func NewParams(targetBaseRewardsRate sdkmath.LegacyDec, poolFunders []string) Params {
	return Params{
		TargetBaseRewardsRate: targetBaseRewardsRate,
		PoolFunders:          poolFunders,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		sdkmath.LegacyZeroDec(),
		nil,
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if err := validateTargetBaseRewardsRate(p.TargetBaseRewardsRate); err != nil {
		return err
	}
	return validatePoolFunders(p.PoolFunders)
}

// IsAuthorizedFunder returns true if addr is in the pool_funders whitelist.
func (p Params) IsAuthorizedFunder(addr string) bool {
	for _, f := range p.PoolFunders {
		if f == addr {
			return true
		}
	}
	return false
}

func validateTargetBaseRewardsRate(v sdkmath.LegacyDec) error {
	if v.IsNil() {
		return fmt.Errorf("target base rewards rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("target base rewards rate cannot be negative: %s", v)
	}

	return nil
}

func validatePoolFunders(funders []string) error {
	seen := make(map[string]struct{}, len(funders))
	for _, f := range funders {
		if _, err := sdk.AccAddressFromBech32(f); err != nil {
			return fmt.Errorf("invalid pool funder address %q: %w", f, err)
		}
		if _, dup := seen[f]; dup {
			return fmt.Errorf("duplicate pool funder address: %s", f)
		}
		seen[f] = struct{}{}
	}
	return nil
}
