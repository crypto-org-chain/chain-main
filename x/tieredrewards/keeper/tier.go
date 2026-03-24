package keeper

import (
	"context"
	stderrors "errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
)

// GetTier returns the tier by id, or ErrTierNotFound when it does not exist.
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

// SetTier validates and persists a tier.
func (k Keeper) SetTier(ctx context.Context, tier types.Tier) error {
	if err := tier.Validate(); err != nil {
		return err
	}
	if err := k.Tiers.Set(ctx, tier.Id, tier); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), tier.Id)
	}
	return nil
}

// DeleteTier removes a tier by ID. Fails if the tier has active positions.
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

// HasTier checks if a tier exists.
func (k Keeper) HasTier(ctx context.Context, id uint32) (bool, error) {
	has, err := k.Tiers.Has(ctx, id)
	if err != nil {
		return false, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), id)
	}
	return has, nil
}

// HasActivePositionsForTier returns true if any positions exist for a tier.
func (k Keeper) HasActivePositionsForTier(ctx context.Context, tierId uint32) (bool, error) {
	count, err := k.GetPositionCountForTier(ctx, tierId)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetPositionCountForTier returns the number of positions for a tier.
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

// increasePositionCount increments the position count for a tier.
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

// decreasePositionCount decrements the position count for a tier.
func (k Keeper) decreasePositionCount(ctx context.Context, id uint32) error {
	count, err := k.GetPositionCountForTier(ctx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count == 1 {
		if err := k.PositionCountByTier.Remove(ctx, id); err != nil {
			return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), id)
		}
		return nil
	}
	if err := k.PositionCountByTier.Set(ctx, id, count-1); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), id)
	}
	return nil
}
