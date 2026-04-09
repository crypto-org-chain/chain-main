package types

import (
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

type Delegation struct {
	Validator           string
	Shares              math.LegacyDec
	BaseRewardsPerShare sdk.DecCoins
}

func NewPosition(id uint64, owner string, tierId uint32, amount math.Int, createdAtHeight int64, delegation Delegation, createdAtTime time.Time) Position {
	return Position{
		Id:                  id,
		Owner:               owner,
		TierId:              tierId,
		Amount:              amount,
		CreatedAtHeight:     uint64(createdAtHeight),
		CreatedAtTime:       createdAtTime,
		Validator:           delegation.Validator,
		DelegatedShares:     delegation.Shares,
		BaseRewardsPerShare: delegation.BaseRewardsPerShare,
		LastBonusAccrual:    createdAtTime,
	}
}

func (p Position) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if p.Amount.IsNil() {
		return fmt.Errorf("amount locked cannot be nil")
	}

	if p.Amount.IsNegative() {
		return fmt.Errorf("amount locked must not be negative: %s", p.Amount)
	}

	if p.IsDelegated() {
		if _, err := sdk.ValAddressFromBech32(p.Validator); err != nil {
			return fmt.Errorf("invalid validator address: %w", err)
		}
		if len(p.BaseRewardsPerShare) > 0 {
			if err := p.BaseRewardsPerShare.Validate(); err != nil {
				return fmt.Errorf("invalid base rewards per share: %w", err)
			}
		}
		if !p.DelegatedShares.IsPositive() {
			return fmt.Errorf("delegated shares must be positive when validator is set")
		}
	} else {
		if !p.DelegatedShares.IsNil() && !p.DelegatedShares.IsZero() {
			return fmt.Errorf("delegated shares must not be set when not delegated")
		}
		if len(p.BaseRewardsPerShare) > 0 {
			return fmt.Errorf("base rewards per share must not be set when not delegated")
		}
		if !p.LastBonusAccrual.IsZero() {
			return fmt.Errorf("last bonus accrual must not be set when not delegated")
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
	p.BaseRewardsPerShare = delegation.BaseRewardsPerShare
	p.LastBonusAccrual = t
}

func (p *Position) UpdateBaseRewardsPerShare(brps sdk.DecCoins) {
	p.BaseRewardsPerShare = brps
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
	p.BaseRewardsPerShare = sdk.DecCoins{}
	p.LastBonusAccrual = time.Time{}
}

func (p Position) HasTriggeredExit() bool {
	return !p.ExitTriggeredAt.IsZero()
}

func (p Position) IsOwner(address string) bool {
	return p.Owner == address
}
