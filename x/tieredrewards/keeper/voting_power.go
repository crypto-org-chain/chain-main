package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// getVotingPowerForAddress returns the tier governance voting power for an address.
// Power = DelegatedShares * BondedTokens / DelegatorShares for each delegated position.
func (k Keeper) getVotingPowerForAddress(ctx context.Context, voter sdk.AccAddress) (math.LegacyDec, error) {
	active, err := k.GetDelegatedPositionsByOwner(ctx, voter)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	power := math.LegacyZeroDec()
	for _, pos := range active {
		if pos.Amount.IsZero() {
			continue
		}
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			continue
		}
		val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
			continue
		}
		if err != nil {
			return math.LegacyZeroDec(), err
		}
		if !val.IsBonded() || val.DelegatorShares.IsZero() {
			continue
		}
		power = power.Add(pos.DelegatedShares.MulInt(val.BondedTokens()).Quo(val.DelegatorShares))
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
		if !pos.IsDelegated() {
			return false, nil
		}
		if pos.Amount.IsZero() {
			return false, nil
		}

		val, ok := vals[pos.Validator]
		if !ok {
			valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
			if err != nil {
				return false, nil
			}
			v, err := k.stakingKeeper.GetValidator(ctx, valAddr)
			if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			val = v
			vals[pos.Validator] = val
		}

		if !val.IsBonded() || val.DelegatorShares.IsZero() {
			return false, nil
		}

		power := pos.DelegatedShares.MulInt(val.BondedTokens()).Quo(val.DelegatorShares)
		total = total.Add(power)
		return false, nil
	})
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	return total, nil
}
