package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Validate performs basic validation of a Tier.
func (t Tier) Validate() error {
	if t.ExitDuration <= 0 {
		return fmt.Errorf("exit duration must be positive")
	}

	if t.BonusApy.IsNil() {
		return fmt.Errorf("bonus apy cannot be nil")
	}

	if t.BonusApy.IsNegative() {
		return fmt.Errorf("bonus apy cannot be negative: %s", t.BonusApy)
	}

	if t.MinLockAmount.IsNil() {
		return fmt.Errorf("min lock amount cannot be nil")
	}

	if !t.MinLockAmount.IsPositive() {
		return fmt.Errorf("min lock amount must be positive: %s", t.MinLockAmount)
	}

	return nil
}

func (t Tier) IsCloseOnly() bool {
	return t.CloseOnly
}

func (t Tier) MeetsMinLockRequirement(amount math.Int) bool {
	return amount.GTE(t.MinLockAmount)
}
