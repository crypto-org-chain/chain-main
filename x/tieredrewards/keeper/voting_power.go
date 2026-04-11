package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"errors"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// validatorProvider captures the staking query needed to convert delegated
// shares into governance voting power.
type validatorProvider interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
}

// hasVotingPower defines whether a position has voting power.
func hasVotingPower(pos types.Position, val stakingtypes.Validator) bool {
	return val.IsBonded() && !val.DelegatorShares.IsZero() && pos.IsDelegated() && !pos.Amount.IsZero() && pos.DelegatedShares.IsPositive()
}

func tierVotingPowerForPosition(
	ctx context.Context,
	sk validatorProvider,
	pos types.Position,
	validatorCache map[string]stakingtypes.Validator,
) (math.LegacyDec, error) {
	val, ok := validatorCache[pos.Validator]
	if !ok {
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return math.LegacyZeroDec(), nil
		}

		val, err = sk.GetValidator(ctx, valAddr)
		if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
			return math.LegacyZeroDec(), nil
		}
		if err != nil {
			return math.LegacyZeroDec(), err
		}

		if validatorCache != nil {
			validatorCache[pos.Validator] = val
		}
	}

	if !hasVotingPower(pos, val) {
		return math.LegacyZeroDec(), nil
	}

	return val.TokensFromShares(pos.DelegatedShares), nil
}

// getVotingPowerForAddress returns the tier governance voting power for an address.
// Power is derived from delegated position shares via validator exchange rates.
func (k Keeper) getVotingPowerForAddress(ctx context.Context, voter sdk.AccAddress) (math.LegacyDec, error) {
	active, err := k.GetDelegatedPositionsByOwner(ctx, voter)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	power := math.LegacyZeroDec()
	vals := make(map[string]stakingtypes.Validator)
	for _, pos := range active {
		posPower, err := tierVotingPowerForPosition(ctx, k.stakingKeeper, pos, vals)
		if err != nil {
			return math.LegacyZeroDec(), err
		}
		power = power.Add(posPower)
	}
	return power, nil
}

func (k Keeper) GetDelegatedPositionsByOwner(ctx context.Context, voter sdk.AccAddress) ([]types.Position, error) {
	positions, err := k.getPositionsByOwner(ctx, voter)
	if err != nil {
		return nil, err
	}

	var active []types.Position
	for _, pos := range positions {
		if pos.IsDelegated() {
			active = append(active, pos)
		}
	}
	return active, nil
}

func (k Keeper) totalDelegatedVotingPower(ctx context.Context) (math.LegacyDec, error) {
	total := math.LegacyZeroDec()
	vals := make(map[string]stakingtypes.Validator)

	err := k.Positions.Walk(ctx, nil, func(_ uint64, pos types.Position) (bool, error) {
		posPower, err := tierVotingPowerForPosition(ctx, k.stakingKeeper, pos, vals)
		if err != nil {
			return false, err
		}
		total = total.Add(posPower)
		return false, nil
	})
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	return total, nil
}
