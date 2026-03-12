package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// MaxBonusAPY is an upper bound on the bonus APY to prevent unreasonable configurations.
var MaxBonusAPY = sdkmath.LegacyNewDec(10) // 1000%

// NewParams creates a new Params instance.
func NewParams(targetBaseRewardsRate sdkmath.LegacyDec, tiers []TierDefinition, bonusDenoms []string) Params {
	return Params{
		TargetBaseRewardsRate: targetBaseRewardsRate,
		Tiers:                 tiers,
		BonusDenoms:           bonusDenoms,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		sdkmath.LegacyZeroDec(),
		[]TierDefinition{},
		[]string{},
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if err := validateTargetBaseRewardsRate(p.TargetBaseRewardsRate); err != nil {
		return err
	}
	return validateTiers(p.Tiers)
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

func validateTiers(tiers []TierDefinition) error {
	seen := make(map[uint32]bool)
	for _, t := range tiers {
		if seen[t.TierId] {
			return fmt.Errorf("duplicate tier id: %d", t.TierId)
		}
		seen[t.TierId] = true
		if t.ExitCommitmentDuration <= 0 {
			return fmt.Errorf("tier %d: exit commitment duration must be positive", t.TierId)
		}
		if t.ExitCommitmentDurationInYears <= 0 {
			return fmt.Errorf("tier %d: exit commitment duration in years must be positive", t.TierId)
		}
		if t.BonusApy.IsNil() || t.BonusApy.IsNegative() {
			return fmt.Errorf("tier %d: bonus APY must be non-negative", t.TierId)
		}
		if t.BonusApy.GT(MaxBonusAPY) {
			return fmt.Errorf("tier %d: bonus APY %s exceeds maximum %s", t.TierId, t.BonusApy, MaxBonusAPY)
		}
		if t.MinLockAmount.IsNil() || t.MinLockAmount.IsNegative() {
			return fmt.Errorf("tier %d: min lock amount must be non-negative", t.TierId)
		}
	}
	return nil
}

// GetTierDefinition returns the TierDefinition for the given tier ID, or an error.
func (p Params) GetTierDefinition(tierId uint32) (TierDefinition, error) {
	for _, t := range p.Tiers {
		if t.TierId == tierId {
			return t, nil
		}
	}
	return TierDefinition{}, fmt.Errorf("tier %d not found", tierId)
}
