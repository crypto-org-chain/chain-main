package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Validate performs basic validation of a TierPosition.
func (p TierPosition) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return fmt.Errorf("invalid owner address: %w", err)
	}

	if p.AmountLocked.IsNil() {
		return fmt.Errorf("amount locked cannot be nil")
	}

	if !p.AmountLocked.IsPositive() {
		return fmt.Errorf("amount locked must be positive: %s", p.AmountLocked)
	}

	if !p.IsDelegated() && !p.DelegatedShares.IsNil() {
		return fmt.Errorf("delegated shares must not be set when not delegated")
	}

	if p.IsDelegated() {
		if _, err := sdk.ValAddressFromBech32(p.Validator); err != nil {
			return fmt.Errorf("invalid validator address: %w", err)
		}

		if !p.DelegatedShares.IsPositive() {
			return fmt.Errorf("delegated shares must be positive when validator is set")
		}
	}

	if !p.IsExiting() && !p.ExitUnlockAt.IsZero() {
		return fmt.Errorf("exit_unlock_at must not be set for a position that is not exiting")
	}

	if p.IsExiting() && !p.ExitUnlockAt.After(p.ExitTriggeredAt) {
		return fmt.Errorf("exit_unlock_at must be after exit_triggered_at")
	}

	if p.CreatedAtHeight == 0 {
		return fmt.Errorf("created_at_height must be positive")
	}

	if p.CreatedAtTime.IsZero() {
		return fmt.Errorf("created_at_time must be non-zero")
	}

	return nil
}

// IsDelegated returns true if the position is delegated to a validator.
func (p TierPosition) IsDelegated() bool {
	return p.Validator != ""
}

// IsExiting returns true if exit has been triggered for this position.
func (p TierPosition) IsExiting() bool {
	return !p.ExitTriggeredAt.IsZero()
}
