package keeper

import (
	"context"
	stderrors "errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
)

func (k Keeper) getPosition(ctx context.Context, id uint64) (types.Position, error) {
	pos, err := k.Positions.Get(ctx, id)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return types.Position{}, errorsmod.Wrapf(types.ErrPositionNotFound, "position id %d", id)
		}
		return types.Position{}, errorsmod.Wrapf(err, "%s (position id %d)", types.ErrPositionStore.Error(), id)
	}
	return pos, nil
}
