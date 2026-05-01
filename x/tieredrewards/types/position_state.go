package types

import (
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type PositionState struct {
	Position
	Delegation *stakingtypes.Delegation
}

func (p PositionState) IsDelegated() bool { return p.Delegation != nil }
