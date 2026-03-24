package keeper

import (
	"context"
	stderrors "errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
)

func (k Keeper) GetTier(ctx context.Context, id uint32) (types.Tier, error) {
	tier, err := k.Tiers.Get(ctx, id)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return types.Tier{}, errorsmod.Wrapf(types.ErrTierNotFound, "tier id %d", id)
		}
		return types.Tier{}, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), id)
	}
	return tier, nil
}

func (k Keeper) SetTier(ctx context.Context, tier types.Tier) error {
	if err := tier.Validate(); err != nil {
		return err
	}
	if err := k.Tiers.Set(ctx, tier.Id, tier); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), tier.Id)
	}
	return nil
}

func (k Keeper) DeleteTier(ctx context.Context, tierId uint32) error {
	hasPositions, err := k.HasActivePositionsForTier(ctx, tierId)
	if err != nil {
		return err
	}
	if hasPositions {
		return types.ErrTierHasActivePositions
	}
	if err := k.Tiers.Remove(ctx, tierId); err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return errorsmod.Wrapf(types.ErrTierNotFound, "tier id %d", tierId)
		}
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), tierId)
	}
	return nil
}

func (k Keeper) HasTier(ctx context.Context, id uint32) (bool, error) {
	has, err := k.Tiers.Has(ctx, id)
	if err != nil {
		return false, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), id)
	}
	return has, nil
}

func (k Keeper) HasActivePositionsForTier(ctx context.Context, tierId uint32) (bool, error) {
	count, err := k.GetPositionCountForTier(ctx, tierId)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (k Keeper) GetPositionCountForTier(ctx context.Context, tierId uint32) (uint64, error) {
	count, err := k.PositionCountByTier.Get(ctx, tierId)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return 0, nil
		}
		return 0, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), tierId)
	}
	return count, nil
}

func (k Keeper) increasePositionCount(ctx context.Context, tierId uint32) error {
	count, err := k.GetPositionCountForTier(ctx, tierId)
	if err != nil {
		return err
	}
	if err := k.PositionCountByTier.Set(ctx, tierId, count+1); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), tierId)
	}
	return nil
}

func (k Keeper) decreasePositionCount(ctx context.Context, id uint32) error {
	count, err := k.GetPositionCountForTier(ctx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count == 1 {
		return k.PositionCountByTier.Remove(ctx, id)
	}
	return k.PositionCountByTier.Set(ctx, id, count-1)
}
