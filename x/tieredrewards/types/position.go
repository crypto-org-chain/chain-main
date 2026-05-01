package types

import (
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

type Delegation struct {
	Validator    string
	Shares       math.LegacyDec
	LastEventSeq uint64
}

func NewPosition(id uint64, owner string, tierId uint32, amount math.Int, createdAtHeight uint64, delegation Delegation, createdAtTime time.Time) Position {
	return Position{
		Id:               id,
		Owner:            owner,
		TierId:           tierId,
		Amount:           amount,
		CreatedAtHeight:  createdAtHeight,
		CreatedAtTime:    createdAtTime,
		Validator:        delegation.Validator,
		DelegatedShares:  delegation.Shares,
		LastBonusAccrual: createdAtTime,
		LastEventSeq:     delegation.LastEventSeq,
		// Delegated positions are only created on bonded validators.
		LastKnownBonded: delegation.Validator != "",
	}
}

func (p Position) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if p.Amount.IsNil() {
		return fmt.Errorf("amount cannot be nil")
	}

	if p.Amount.IsNegative() {
		return fmt.Errorf("amount must not be negative: %s", p.Amount)
	}

	if p.IsDelegated() {
		if _, err := sdk.ValAddressFromBech32(p.Validator); err != nil {
			return fmt.Errorf("invalid validator address: %w", err)
		}
		if !p.DelegatedShares.IsPositive() {
			return fmt.Errorf("delegated shares must be positive when validator is set")
		}
		if !p.Amount.IsZero() {
			return fmt.Errorf("amount must be zero for delegated positions")
		}
	} else {
		if !p.DelegatedShares.IsNil() && !p.DelegatedShares.IsZero() {
			return fmt.Errorf("delegated shares must not be set when not delegated")
		}
		if !p.LastBonusAccrual.IsZero() {
			return fmt.Errorf("last bonus accrual must not be set when not delegated")
		}
		if p.LastEventSeq != 0 {
			return fmt.Errorf("last event seq must not be set when not delegated")
		}
		if p.LastKnownBonded {
			return fmt.Errorf("last known bonded must not be true when not delegated")
		}
	}

	if p.ExitTriggeredAt.IsZero() && !p.ExitUnlockAt.IsZero() {
		return fmt.Errorf("exit_unlock_at must not be set for a position that is not exiting")
	}

	if !p.ExitTriggeredAt.IsZero() && !p.ExitUnlockAt.After(p.ExitTriggeredAt) {
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

func (p Position) IsDelegated() bool {
	return p.Validator != ""
}

func (p Position) IsExiting(blockTime time.Time) bool {
	return !p.ExitTriggeredAt.IsZero() && !blockTime.Before(p.ExitTriggeredAt) && blockTime.Before(p.ExitUnlockAt)
}

func (p Position) CompletedExitLockDuration(blockTime time.Time) bool {
	return !p.ExitUnlockAt.IsZero() && !blockTime.Before(p.ExitUnlockAt)
}

func (p *Position) WithDelegation(delegation Delegation, t time.Time) {
	p.Validator = delegation.Validator
	p.DelegatedShares = delegation.Shares
	p.LastEventSeq = delegation.LastEventSeq
	// Position is delegated as a whole
	p.Amount = math.ZeroInt()
	p.LastBonusAccrual = t
	// Delegation is only allowed to bonded validators.
	p.LastKnownBonded = true
}

func (p *Position) TriggerExit(blockTime time.Time, duration time.Duration) {
	p.ExitTriggeredAt = blockTime
	p.ExitUnlockAt = blockTime.Add(duration)
}

func (p *Position) ClearExit(blockTime time.Time) {
	p.ExitTriggeredAt = time.Time{}
	p.ExitUnlockAt = time.Time{}
	if p.IsDelegated() {
		// required so that positions who clear exit after exit lock duration won't have extra bonus accrued
		p.LastBonusAccrual = blockTime
	}
}

func (p *Position) UpdateLastBonusAccrual(t time.Time) {
	p.LastBonusAccrual = t
}

func (p *Position) UpdateAmount(amount math.Int) {
	p.Amount = amount
}

func (p *Position) UpdateDelegatedShares(shares math.LegacyDec) {
	p.DelegatedShares = shares
}

func (p *Position) ClearDelegation() {
	p.Validator = ""
	p.DelegatedShares = math.LegacyZeroDec()
	p.LastBonusAccrual = time.Time{}
	p.LastEventSeq = 0
	p.LastKnownBonded = false
}

func (p Position) HasTriggeredExit() bool {
	return !p.ExitTriggeredAt.IsZero()
}

func (p Position) IsOwner(address string) bool {
	return p.Owner == address
}

func (p Position) ExitWithFullDelegation(amount, tokenValue math.Int) bool {
	return amount.Equal(tokenValue)
}

func (p *Position) UpdateLastEventSeq(seq uint64) {
	p.LastEventSeq = seq
}

func (p *Position) UpdateLastKnownBonded(bonded bool) {
	p.LastKnownBonded = bonded
}

func GetDelegatorAddress(id uint64) sdk.AccAddress {
	return authtypes.NewModuleAddress(fmt.Sprintf("tieredrewards/position/%d", id))
}

func (p *Position) ToPositionResponse(tokenValue math.Int) PositionResponse {
	return PositionResponse{
		Id:               p.Id,
		Owner:            p.Owner,
		TierId:           p.TierId,
		Amount:           tokenValue,
		Validator:        p.Validator,
		DelegatedShares:  p.DelegatedShares,
		LastBonusAccrual: p.LastBonusAccrual,
		ExitTriggeredAt:  p.ExitTriggeredAt,
		ExitUnlockAt:     p.ExitUnlockAt,
		CreatedAtHeight:  p.CreatedAtHeight,
		CreatedAtTime:    p.CreatedAtTime,
	}
}
