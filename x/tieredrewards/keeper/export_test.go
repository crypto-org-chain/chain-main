package keeper

import (
	"context"

	"cosmossdk.io/math"
)

// TransferDelegation exposes transferDelegation for black-box tests in package keeper_test.
func (k Keeper) TransferDelegation(ctx context.Context, delegatorAddr, validatorAddr string, amount math.Int) (math.LegacyDec, error) {
	return k.transferDelegation(ctx, delegatorAddr, validatorAddr, amount)
}
