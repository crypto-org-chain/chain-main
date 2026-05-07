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

func NewPosition(id uint64, owner string, tierId uint32, createdAtHeight, lastEventSeq uint64, lastBonusAccrual time.Time, lastKnownBonded bool, createdAtTime time.Time) Position {
	return Position{
		Id:               id,
		Owner:            owner,
		TierId:           tierId,
		CreatedAtHeight:  createdAtHeight,
		CreatedAtTime:    createdAtTime,
		LastEventSeq:     lastEventSeq,
		LastBonusAccrual: lastBonusAccrual,
		LastKnownBonded:  lastKnownBonded,
	}
}

func (p Position) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
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

func (p Position) IsExiting(blockTime time.Time) bool {
	return !p.ExitTriggeredAt.IsZero() && !blockTime.Before(p.ExitTriggeredAt) && blockTime.Before(p.ExitUnlockAt)
}

func (p Position) CompletedExitLockDuration(blockTime time.Time) bool {
	return !p.ExitUnlockAt.IsZero() && !blockTime.Before(p.ExitUnlockAt)
}

func (p *Position) UpdateBonusCheckpoints(lastEventSeq uint64, t time.Time) {
	p.LastEventSeq = lastEventSeq
	p.LastBonusAccrual = t
	p.LastKnownBonded = true
}

func (p *Position) TriggerExit(blockTime time.Time, duration time.Duration) {
	p.ExitTriggeredAt = blockTime
	p.ExitUnlockAt = blockTime.Add(duration)
}

func (p *Position) UpdateLastBonusAccrual(t time.Time) {
	p.LastBonusAccrual = t
}

func (p *Position) ResetBonusCheckpoints() {
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

func (p Position) ExitWithFullDelegation(amount, positionAmount math.Int) bool {
	return amount.Equal(positionAmount)
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
