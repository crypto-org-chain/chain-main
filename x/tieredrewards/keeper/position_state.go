package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) loadPosition(ctx context.Context, posId uint64) (types.PositionState, error) {
	pos, err := k.getPosition(ctx, posId)
	if err != nil {
		return types.PositionState{}, err
	}
	// A position has at most one delegation.
	dels, err := k.stakingKeeper.GetDelegatorDelegations(ctx, types.GetDelegatorAddress(posId), 1)
	if err != nil {
		return types.PositionState{}, err
	}
	state := types.PositionState{Position: pos}
	if len(dels) > 0 {
		d := dels[0]
		state.Delegation = &d
	}
	return state, nil
}

func (k Keeper) positionAmount(ctx context.Context, pos types.PositionState) (math.Int, error) {
	if pos.IsDelegated() {
		return k.delegatedAmount(ctx, pos)
	}
	return k.undelegatedAmount(ctx, pos)
}

func (k Keeper) delegatedAmount(ctx context.Context, pos types.PositionState) (math.Int, error) {
	valAddr, err := sdk.ValAddressFromBech32(pos.Delegation.ValidatorAddress)
	if err != nil {
		return math.Int{}, err
	}
	return k.reconcileAmountFromShares(ctx, valAddr, pos.Delegation.Shares)
}

func (k Keeper) undelegatedAmount(ctx context.Context, pos types.PositionState) (math.Int, error) {
	delAddr := types.GetDelegatorAddress(pos.Id)
	ubds, err := k.stakingKeeper.GetUnbondingDelegations(ctx, delAddr, 1)
	if err != nil {
		return math.Int{}, err
	}
	// A position has at most one UD with one entry.
	if len(ubds) > 0 && len(ubds[0].Entries) > 0 {
		return ubds[0].Entries[0].Balance, nil
	}
	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return math.Int{}, err
	}
	return k.bankKeeper.GetBalance(ctx, delAddr, bondDenom).Amount, nil
}
