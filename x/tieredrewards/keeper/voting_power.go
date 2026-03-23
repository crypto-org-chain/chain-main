package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetVotingPowerForAddress returns the tier governance voting power for a given
// address. Power is computed using the same shares-to-tokens formula as the
// governance tally: DelegatedShares * BondedTokens / DelegatorShares.
// Only active positions (delegated, not exiting, validator bonded) contribute.
func (k Keeper) GetVotingPowerForAddress(ctx context.Context, voter sdk.AccAddress) (math.LegacyDec, error) {
	active, err := k.GetActiveDelegatedPositionsByOwner(ctx, voter)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	power := math.LegacyZeroDec()
	for _, pos := range active {
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

// GetActiveDelegatedPositionsByOwner returns all positions owned by voter that
// are currently delegated to a validator and have NOT triggered an exit.
// The governance tally uses these positions to deduct each position's
// DelegatedShares from the corresponding validator's DelegatorDeductions,
// preventing the tiered-rewards module account's delegation from being
// double-counted in the validator second-pass tally.
func (k Keeper) GetActiveDelegatedPositionsByOwner(ctx context.Context, voter sdk.AccAddress) ([]types.Position, error) {
	positions, err := k.GetPositionsByOwner(ctx, voter)
	if err != nil {
		return nil, err
	}

	var active []types.Position
	for _, pos := range positions {
		if pos.IsActiveForGovernance() {
			active = append(active, pos)
		}
	}
	return active, nil
}

// TotalDelegatedVotingPower returns the sum of locked Amount for all active
// delegated positions (delegated and not exiting). Tier delegations already
// flow into TotalBondedTokens via the module account, so this value is not
// added to the governance quorum denominator; it is provided for informational
// purposes only.
//
// Note: this returns the nominal locked amount (pos.Amount), not the
// shares-to-tokens value used by the governance tally. The two may diverge
// after validator slashing, but since this is informational-only the
// simpler formula is intentional.
func (k Keeper) TotalDelegatedVotingPower(ctx context.Context) (math.LegacyDec, error) {
	total := math.LegacyZeroDec()
	err := k.Positions.Walk(ctx, nil, func(_ uint64, pos types.Position) (bool, error) {
		if pos.IsActiveForGovernance() {
			total = total.Add(math.LegacyNewDecFromInt(pos.Amount))
		}
		return false, nil
	})
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	return total, nil
}
