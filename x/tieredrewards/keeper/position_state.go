package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) getPositionState(ctx context.Context, posId uint64) (types.PositionState, error) {
	pos, err := k.getPosition(ctx, posId)
	if err != nil {
		return types.PositionState{}, err
	}
	del, err := k.getDelegation(ctx, posId)
	if err != nil {
		return types.PositionState{}, err
	}
	return types.PositionState{Position: pos, Delegation: del}, nil
}

func (k Keeper) getPositionAmount(ctx context.Context, pos types.PositionState) (math.Int, error) {
	if pos.IsDelegated() {
		return k.getDelegatedAmount(ctx, pos)
	}
	return k.getUndelegatedAmount(ctx, pos.Position)
}

func (k Keeper) getDelegatedAmount(ctx context.Context, pos types.PositionState) (math.Int, error) {
	valAddr, err := sdk.ValAddressFromBech32(pos.Delegation.ValidatorAddress)
	if err != nil {
		return math.Int{}, err
	}
	return k.reconcileAmountFromShares(ctx, valAddr, pos.Delegation.Shares)
}

func (k Keeper) getUndelegatedAmount(ctx context.Context, pos types.Position) (math.Int, error) {
	delAddr := types.GetDelegatorAddress(pos.Id)
	ubds, err := k.stakingKeeper.GetUnbondingDelegations(ctx, delAddr, 1)
	if err != nil {
		return math.Int{}, err
	}
	if len(ubds) > 0 && len(ubds[0].Entries) > 0 {
		return ubds[0].Entries[0].Balance, nil
	}
	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return math.Int{}, err
	}
	return k.bankKeeper.GetBalance(ctx, delAddr, bondDenom).Amount, nil
}

// GetPositionStatesByOwner returns each owned position paired with its
// staking delegation (if any).
// Used by gov tally, skip positions that are not found to prevent endblocker halting.
func (k Keeper) GetPositionStatesByOwner(ctx context.Context, owner sdk.AccAddress) ([]types.PositionState, error) {
	ids, err := k.getPositionsIdsByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	states := make([]types.PositionState, 0, len(ids))
	for _, id := range ids {
		state, err := k.getPositionState(ctx, id)
		if errors.Is(err, types.ErrPositionNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}
