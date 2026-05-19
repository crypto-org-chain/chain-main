package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func positionVotingPower(
	pos types.PositionState,
	bondedVals map[string]v1.ValidatorGovInfo,
) math.LegacyDec {
	if !pos.IsDelegated() {
		return math.LegacyZeroDec()
	}
	val, ok := bondedVals[pos.Delegation.ValidatorAddress]
	if !ok || val.DelegatorShares.IsZero() {
		return math.LegacyZeroDec()
	}
	return pos.Delegation.Shares.MulInt(val.BondedTokens).Quo(val.DelegatorShares)
}

// getCurrentValidators gets the current bonded validators from the staking keeper. Same implementation as in gov keeper.
// decided against importing gov keeper just for this function.
func (k Keeper) getCurrentValidators(ctx context.Context) (map[string]v1.ValidatorGovInfo, error) {
	currValidators := make(map[string]v1.ValidatorGovInfo)
	if err := k.stakingKeeper.IterateBondedValidatorsByPower(ctx, func(index int64, validator stakingtypes.ValidatorI) (stop bool) {
		valBz, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(validator.GetOperator())
		if err != nil {
			return false
		}
		currValidators[validator.GetOperator()] = v1.NewValidatorGovInfo(
			valBz,
			validator.GetBondedTokens(),
			validator.GetDelegatorShares(),
			math.LegacyZeroDec(),
			v1.WeightedVoteOptions{},
		)

		return false
	}); err != nil {
		return nil, err
	}

	return currValidators, nil
}

// getVotingPowerByOwner returns the tier governance voting power for an address.
// Power is derived from each position's delegation.
func (k Keeper) getVotingPowerByOwner(ctx context.Context, owner sdk.AccAddress) (math.LegacyDec, error) {
	states, err := k.GetPositionStatesByOwner(ctx, owner)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	vals, err := k.getCurrentValidators(ctx)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	power := math.LegacyZeroDec()
	for _, p := range states {
		power = power.Add(positionVotingPower(p, vals))
	}
	return power, nil
}

func (k Keeper) totalDelegatedVotingPower(ctx context.Context) (math.LegacyDec, error) {
	total := math.LegacyZeroDec()

	vals, err := k.getCurrentValidators(ctx)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	err = k.Positions.Walk(ctx, nil, func(posId uint64, _ types.Position) (bool, error) {
		state, err := k.getPositionState(ctx, posId)
		if err != nil {
			return true, err
		}
		total = total.Add(positionVotingPower(state, vals))
		return false, nil
	})
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	return total, nil
}
