package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// GetTier returns a tier by ID.
func (k Keeper) GetTier(ctx context.Context, tierId uint32) (types.Tier, error) {
	tier, err := k.Tiers.Get(ctx, tierId)
	if err != nil {
		return types.Tier{}, err
	}
	return tier, nil
}

// SetTier stores a tier. Validates before saving.
func (k Keeper) SetTier(ctx context.Context, tier types.Tier) error {
	if err := tier.Validate(); err != nil {
		return err
	}
	return k.Tiers.Set(ctx, tier.Id, tier)
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
	return k.Tiers.Remove(ctx, tierId)
}

// HasTier checks if a tier exists.
func (k Keeper) HasTier(ctx context.Context, id uint32) (bool, error) {
	return k.Tiers.Has(ctx, id)
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
		if errors.Is(err, collections.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

// increasePositionCount increments the position count for a tier.
func (k Keeper) increasePositionCount(ctx context.Context, tierId uint32) error {
	count, err := k.GetPositionCountForTier(ctx, tierId)
	if err != nil {
		return err
	}
	return k.PositionCountByTier.Set(ctx, tierId, count+1)
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
		return k.PositionCountByTier.Remove(ctx, id)
	}
	return k.PositionCountByTier.Set(ctx, id, count-1)
}
