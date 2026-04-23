package keeper

import (
	"context"
	stderrors "errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) getTier(ctx context.Context, id uint32) (types.Tier, error) {
	tier, err := k.Tiers.Get(ctx, id)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return types.Tier{}, errorsmod.Wrapf(types.ErrTierNotFound, "tier id %d", id)
		}
		return types.Tier{}, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), id)
	}
	return tier, nil
}

// SetTier writes a tier after validation. Used by governance messages, genesis, and chain upgrades.
func (k Keeper) SetTier(ctx context.Context, tier types.Tier) error {
	if err := tier.Validate(); err != nil {
		return err
	}
	if err := k.Tiers.Set(ctx, tier.Id, tier); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), tier.Id)
	}
	return nil
}

func (k Keeper) deleteTier(ctx context.Context, tierId uint32) error {
	hasPositions, err := k.hasPositionsForTier(ctx, tierId)
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

func (k Keeper) hasTier(ctx context.Context, id uint32) (bool, error) {
	has, err := k.Tiers.Has(ctx, id)
	if err != nil {
		return false, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrTierStore.Error(), id)
	}
	return has, nil
}

func (k Keeper) hasPositionsForTier(ctx context.Context, tierId uint32) (bool, error) {
	count, err := k.getPositionCountForTier(ctx, tierId)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (k Keeper) getPositionCountForTier(ctx context.Context, tierId uint32) (uint64, error) {
	count, err := k.PositionCountByTier.Get(ctx, tierId)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return 0, nil
		}
		return 0, errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), tierId)
	}
	return count, nil
}

func (k Keeper) increasePositionCountForTier(ctx context.Context, tierId uint32) error {
	count, err := k.getPositionCountForTier(ctx, tierId)
	if err != nil {
		return err
	}
	if err := k.PositionCountByTier.Set(ctx, tierId, count+1); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), tierId)
	}
	return nil
}

func (k Keeper) decreasePositionCountForTier(ctx context.Context, id uint32) error {
	count, err := k.getPositionCountForTier(ctx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count == 1 {
		return k.PositionCountByTier.Remove(ctx, id)
	}

	if err := k.PositionCountByTier.Set(ctx, id, count-1); err != nil {
		return errorsmod.Wrapf(err, "%s (tier id %d)", types.ErrPositionCountStore.Error(), id)
	}

	return nil
}

func (k Keeper) getPositionCountForValidator(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	count, err := k.PositionCountByValidator.Get(ctx, valAddr)
	if stderrors.Is(err, collections.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, errorsmod.Wrapf(err, "%s (validator %s)", types.ErrValidatorPositionCountStore.Error(), valAddr)
	}
	return count, nil
}

func (k Keeper) increasePositionCountForValidator(ctx context.Context, valAddr sdk.ValAddress) error {
	count, err := k.getPositionCountForValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if err := k.PositionCountByValidator.Set(ctx, valAddr, count+1); err != nil {
		return errorsmod.Wrapf(err, "%s (validator %s)", types.ErrValidatorPositionCountStore.Error(), valAddr)
	}
	return nil
}

func (k Keeper) decreasePositionCountForValidator(ctx context.Context, valAddr sdk.ValAddress) error {
	count, err := k.getPositionCountForValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count == 1 {
		return k.PositionCountByValidator.Remove(ctx, valAddr)
	}
	if err := k.PositionCountByValidator.Set(ctx, valAddr, count-1); err != nil {
		return errorsmod.Wrapf(err, "%s (validator %s)", types.ErrValidatorPositionCountStore.Error(), valAddr)
	}
	return nil
}
