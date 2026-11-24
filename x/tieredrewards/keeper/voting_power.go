package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	addresscodec "cosmossdk.io/core/address"
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

// bondedValidatorSource is the staking keeper subset needed to snapshot bonded validators.
type bondedValidatorSource interface {
	ValidatorAddressCodec() addresscodec.Codec
	IterateBondedValidatorsByPower(ctx context.Context, fn func(index int64, validator stakingtypes.ValidatorI) (stop bool)) error
}

// bondedValidatorGovInfos returns the bonded validators by operator address and their total power, as the gov tally does.
func bondedValidatorGovInfos(ctx context.Context, sk bondedValidatorSource) (map[string]v1.ValidatorGovInfo, math.Int, error) {
	validators := make(map[string]v1.ValidatorGovInfo)
	totalPower := math.ZeroInt()
	if err := sk.IterateBondedValidatorsByPower(ctx, func(_ int64, validator stakingtypes.ValidatorI) (stop bool) {
		valBz, err := sk.ValidatorAddressCodec().StringToBytes(validator.GetOperator())
		if err != nil {
			return false
		}
		power := validator.GetValidatorPower()
		validators[validator.GetOperator()] = v1.NewValidatorGovInfo(
			valBz,
			power,
			validator.GetDelegatorShares(),
			math.LegacyZeroDec(),
			v1.WeightedVoteOptions{},
		)
		totalPower = totalPower.Add(power)

		return false
	}); err != nil {
		return nil, math.ZeroInt(), err
	}

	return validators, totalPower, nil
}

// getVotingPowerByOwner returns the tier governance voting power for an address.
// Power is derived from each position's delegation.
func (k Keeper) getVotingPowerByOwner(ctx context.Context, owner sdk.AccAddress) (math.LegacyDec, error) {
	states, err := k.GetPositionStatesByOwner(ctx, owner)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	vals, _, err := bondedValidatorGovInfos(ctx, k.stakingKeeper)
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

	vals, _, err := bondedValidatorGovInfos(ctx, k.stakingKeeper)
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
