package tieredrewards

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	return k.TopUpBaseRewards(ctx)
}

// EndBlocker processes completed unbondings for tier positions.
func EndBlocker(ctx context.Context, k keeper.Keeper) error {
	return k.ProcessCompletedUnbondings(ctx)
}
