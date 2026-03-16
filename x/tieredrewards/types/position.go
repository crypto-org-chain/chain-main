package types

import (
	"fmt"
	time "time"

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

		// It is possible to have no base rewards per share when the position is created
		// if the position is the first one delegated to a validator (no base rewards accrued yet)
		// Decided against storing Coins with 0 amount as it will fail validation
		if len(p.BaseRewardsPerShare) > 0 {
			if err := p.BaseRewardsPerShare.Validate(); err != nil {
				return fmt.Errorf("invalid base rewards per share: %w", err)
			}
		}

		if !p.DelegatedShares.IsPositive() {
			return fmt.Errorf("delegated shares must be positive when validator is set")
		}
	}

	if p.ExitTriggeredAt.IsZero() && !p.ExitUnlockAt.IsZero() {
		return fmt.Errorf("exit_unlock_at must not be set for a position that has not")
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
	return !blockTime.Before(p.ExitTriggeredAt) && blockTime.Before(p.ExitUnlockAt)
}

func (p Position) HasExited(blockTime time.Time) bool {
	return !blockTime.Before(p.ExitUnlockAt)
}

func (p *Position) WithDelegation(delegation Delegation, blockTime time.Time) {
	p.Validator = delegation.Validator
	p.DelegatedShares = delegation.Shares
	p.UpdateBaseRewardsPerShare(delegation.BaseRewardsPerShare)
	p.LastBonusAccrual = blockTime
}

func (p *Position) UpdateBaseRewardsPerShare(brps sdk.DecCoins) {
	p.BaseRewardsPerShare = brps
}

func (p *Position) TriggerExit(blockTime time.Time, duration time.Duration) {
	p.ExitTriggeredAt = blockTime
	p.ExitUnlockAt = blockTime.Add(duration)
}
