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
	LastEventSeq        uint64
}

func NewPosition(id uint64, owner string, tierId uint32, undelegatedAmount math.Int, createdAtHeight uint64, delegation Delegation, createdAtTime time.Time) Position {
	return Position{
		Id:                  id,
		Owner:               owner,
		TierId:              tierId,
		UndelegatedAmount:   undelegatedAmount,
		CreatedAtHeight:     createdAtHeight,
		CreatedAtTime:       createdAtTime,
		Validator:           delegation.Validator,
		DelegatedShares:     delegation.Shares,
		BaseRewardsPerShare: delegation.BaseRewardsPerShare,
		LastBonusAccrual:    createdAtTime,
		LastEventSeq:        delegation.LastEventSeq,
	}
}

func (p Position) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if p.UndelegatedAmount.IsNil() {
		return fmt.Errorf("undelegated amount cannot be nil")
	}

	if p.UndelegatedAmount.IsNegative() {
		return fmt.Errorf("undelegated amount must not be negative: %s", p.UndelegatedAmount)
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
		if !p.UndelegatedAmount.IsZero() {
			return fmt.Errorf("undelegated amount must be zero for delegated positions")
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
		if p.LastEventSeq != 0 {
			return fmt.Errorf("last event seq must be set when not delegated")
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
	p.LastEventSeq = delegation.LastEventSeq
	// Position is delegated as a whole
	p.UndelegatedAmount = math.ZeroInt()
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

func (p *Position) UpdateUndelegatedAmount(amount math.Int) {
	p.UndelegatedAmount = amount
}

func (p *Position) UpdateDelegatedShares(shares math.LegacyDec) {
	p.DelegatedShares = shares
}

func (p *Position) ClearDelegation() {
	p.Validator = ""
	p.DelegatedShares = math.LegacyZeroDec()
	p.BaseRewardsPerShare = sdk.DecCoins{}
	p.LastBonusAccrual = time.Time{}
	p.LastEventSeq = 0
}

func (p Position) HasTriggeredExit() bool {
	return !p.ExitTriggeredAt.IsZero()
}

func (p Position) IsOwner(address string) bool {
	return p.Owner == address
}

func (p Position) ExitWithFullDelegation(amount math.Int, tokenValue math.Int) bool {
	return amount.Equal(tokenValue)
}

func (p *Position) UpdateLastEventSeq(seq uint64) {
	p.LastEventSeq = seq
}

