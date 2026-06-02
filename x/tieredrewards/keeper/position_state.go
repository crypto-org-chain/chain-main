package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (k Keeper) getPositionState(ctx context.Context, posId uint64) (types.PositionState, error) {
	pos, err := k.getPosition(ctx, posId)
	if err != nil {
		return types.PositionState{}, err
	}
	del, err := k.getDelegation(ctx, pos.DelegatorAddress)
	if err != nil {
		return types.PositionState{}, err
	}
	return types.PositionState{Position: pos, Delegation: del}, nil
}

func (k Keeper) getPositionAmount(ctx context.Context, pos types.PositionState) (math.Int, error) {
	if pos.IsDelegated() {
		return k.getDelegatedAmount(ctx, pos.Delegation)
	}
	return k.getUndelegatedAmount(ctx, pos.DelegatorAddress)
}

func (k Keeper) getDelegatedAmount(ctx context.Context, del *stakingtypes.Delegation) (math.Int, error) {
	valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
	if err != nil {
		return math.Int{}, err
	}
	return k.reconcileAmountFromShares(ctx, valAddr, del.Shares)
}

func (k Keeper) getUndelegatedAmount(ctx context.Context, delegatorAddress string) (math.Int, error) {
	delAddr, err := sdk.AccAddressFromBech32(delegatorAddress)
	if err != nil {
		return math.Int{}, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid delegator address")
	}
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
	return k.bankKeeper.SpendableCoins(ctx, delAddr).AmountOf(bondDenom), nil
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
