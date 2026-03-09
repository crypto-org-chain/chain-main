package tieredrewards

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
)

// BeginBlocker is called at the beginning of every block.
func BeginBlocker(_ context.Context, _ keeper.Keeper) error {
	return nil
}
