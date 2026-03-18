package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetVotingPowerForAddress returns the tier voting power for a given address.
// It sums Position.Amount for all delegated positions owned by the address.
// Only positions with a non-empty Validator field (currently delegated) count.
func (k Keeper) GetVotingPowerForAddress(ctx context.Context, voter sdk.AccAddress) (math.LegacyDec, error) {
	positions, err := k.GetPositionsByOwner(ctx, voter)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	power := math.LegacyZeroDec()
	for _, pos := range positions {
		if pos.IsDelegated() {
			power = power.Add(math.LegacyNewDecFromInt(pos.Amount))
		}
	}
	return power, nil
}

// TotalDelegatedVotingPower returns the sum of Amount for all delegated positions.
// Used to include tier-delegated supply in the quorum denominator.
func (k Keeper) TotalDelegatedVotingPower(ctx context.Context) (math.LegacyDec, error) {
	total := math.LegacyZeroDec()
	err := k.Positions.Walk(ctx, nil, func(_ uint64, pos types.Position) (bool, error) {
		if pos.IsDelegated() {
			total = total.Add(math.LegacyNewDecFromInt(pos.Amount))
		}
		return false, nil
	})
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	return total, nil
}
