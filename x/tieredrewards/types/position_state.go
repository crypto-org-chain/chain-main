package types

import (
	"fmt"
	"time"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// PositionState pairs a stored Position with its live staking Delegation.
// Delegation is nil for undelegated positions.
type PositionState struct {
	Position
	Delegation *stakingtypes.Delegation
}

func (p PositionState) IsDelegated() bool { return p.Delegation != nil }

func (p PositionState) Validate() error {
	if err := p.Position.Validate(); err != nil {
		return err
	}
	if p.IsDelegated() {
		if _, err := sdk.ValAddressFromBech32(p.Delegation.ValidatorAddress); err != nil {
			return fmt.Errorf("invalid validator address: %w", err)
		}
		expectedDelAddr := GetDelegatorAddress(p.Id).String()
		if p.Delegation.DelegatorAddress != expectedDelAddr {
			return fmt.Errorf(
				"delegation delegator address %q does not match expected %q for position %d",
				p.Delegation.DelegatorAddress, expectedDelAddr, p.Id,
			)
		}
		if !p.Delegation.Shares.IsPositive() {
			return fmt.Errorf("delegated shares must be positive when position is delegated")
		}
		if p.LastBonusAccrual.IsZero() {
			return fmt.Errorf("last_bonus_accrual must be non-zero when position is delegated")
		}
	} else {
		if !p.LastBonusAccrual.IsZero() {
			return fmt.Errorf("last_bonus_accrual must be zero when position is undelegated")
		}
		if p.LastEventSeq != 0 {
			return fmt.Errorf("last_event_seq must be zero when position is undelegated")
		}
		if p.LastKnownBonded {
			return fmt.Errorf("last_known_bonded must be false when position is undelegated")
		}
	}
	return nil
}

func (p *PositionState) ClearExit(blockTime time.Time) {
	p.ExitTriggeredAt = time.Time{}
	p.ExitUnlockAt = time.Time{}
	if p.IsDelegated() {
		// required so that positions who clear exit after exit lock duration won't have extra bonus accrued
		p.LastBonusAccrual = blockTime
	}
}

func (p PositionState) ToPositionResponse(positionAmount math.Int) PositionResponse {
	resp := PositionResponse{
		Id:              p.Id,
		Owner:           p.Owner,
		TierId:          p.TierId,
		Amount:          positionAmount,
		DelegatedShares: math.LegacyZeroDec(),
		ExitTriggeredAt: p.ExitTriggeredAt,
		ExitUnlockAt:    p.ExitUnlockAt,
		CreatedAtHeight: p.CreatedAtHeight,
		CreatedAtTime:   p.CreatedAtTime,
	}
	if p.IsDelegated() {
		resp.Validator = p.Delegation.ValidatorAddress
		resp.DelegatedShares = p.Delegation.Shares
	}
	return resp
}
