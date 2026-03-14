package types

import (
	"fmt"
	time "time"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func NewBasePosition(id uint64, owner string, tierId uint32, amount math.Int, createdAtHeight int64, createdAtTime time.Time) Position {
	return Position{
		Id:              id,
		Owner:           owner,
		TierId:          tierId,
		Amount:          amount,
		CreatedAtHeight: uint64(createdAtHeight),
		CreatedAtTime:   createdAtTime,
	}

}

// Validate performs basic validation of a Position.
func (p Position) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if p.Amount.IsNil() {
		return fmt.Errorf("amount locked cannot be nil")
	}

	if !p.Amount.IsPositive() {
		return fmt.Errorf("amount locked must be positive: %s", p.Amount)
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
func (p Position) IsDelegated() bool {
	return p.Validator != ""
}

// IsExiting returns true if exit has been triggered for this position.
func (p Position) IsExiting() bool {
	return !p.ExitTriggeredAt.IsZero()
}

func (p *Position) InitDelegation(validator string, shares math.LegacyDec, blockTime time.Time) {
	p.Validator = validator
	p.DelegatedShares = shares
	p.LastBonusAccrual = blockTime
}

func (p *Position) TriggerExit(blockTime time.Time, duration time.Duration) {
	p.ExitTriggeredAt = blockTime
	p.ExitUnlockAt = blockTime.Add(duration)
}
