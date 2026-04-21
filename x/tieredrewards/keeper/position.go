package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (k Keeper) getPosition(ctx context.Context, id uint64) (types.Position, error) {
	pos, err := k.Positions.Get(ctx, id)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.Position{}, errorsmod.Wrapf(types.ErrPositionNotFound, "position id %d", id)
		}
		return types.Position{}, errorsmod.Wrapf(err, "%s (position id %d)", types.ErrPositionStore.Error(), id)
	}

	if !pos.IsDelegated() {
		return pos, nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return types.Position{}, err
	}

	reconciledAmount, err := k.reconcileAmountFromShares(ctx, valAddr, pos.DelegatedShares)
	if err != nil {
		// Keep the stored amount as a fallback if validator metadata is unavailable.
		if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
			return pos, nil
		}
		return types.Position{}, err
	}

	pos.UpdateAmount(reconciledAmount)
	return pos, nil
}

func (k Keeper) createPosition(
	ctx context.Context,
	owner string,
	tier types.Tier,
	amount math.Int,
	delegation types.Delegation,
	triggerExitImmediately bool,
) (types.Position, error) {
	id, err := k.NextPositionId.Next(ctx)
	if err != nil {
		return types.Position{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()
	blockHeight := uint64(sdkCtx.BlockHeight())

	pos := types.NewPosition(id, owner, tier.Id, amount, blockHeight, delegation, blockTime)

	if triggerExitImmediately {
		pos.TriggerExit(blockTime, tier.ExitDuration)
	}

	if err := k.setPosition(ctx, pos); err != nil {
		return types.Position{}, err
	}

	return pos, nil
}

// LockFunds locks the desired amount of funds into a position.
func (k Keeper) lockFunds(ctx context.Context, owner string, amount math.Int) error {
	ownerAddr, err := sdk.AccAddressFromBech32(owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}

	return k.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, types.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, amount)))
}

// SetPosition stores a position, validates it, and maintains secondary indexes.
func (k Keeper) setPosition(ctx context.Context, pos types.Position) error {
	if err := pos.Validate(); err != nil {
		return err
	}
	oldPos, err := k.Positions.Get(ctx, pos.Id)
	isNew := errors.Is(err, collections.ErrNotFound)

	if !isNew && err != nil {
		return errorsmod.Wrapf(err, "%s (position id %d)", types.ErrPositionStore.Error(), pos.Id)
	}

	if err := k.Positions.Set(ctx, pos.Id, pos); err != nil {
		return err
	}

	if isNew {
		owner, err := sdk.AccAddressFromBech32(pos.Owner)
		if err != nil {
			return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
		}
		if err := k.PositionsByOwner.Set(ctx, collections.Join(owner, pos.Id)); err != nil {
			return err
		}
		if err := k.PositionsByTier.Set(ctx, collections.Join(pos.TierId, pos.Id)); err != nil {
			return err
		}
		if pos.IsDelegated() {
			valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
			if err != nil {
				return err
			}
			if err := k.PositionsByValidator.Set(ctx, collections.Join(valAddr, pos.Id)); err != nil {
				return err
			}
		}
		if err := k.increasePositionCount(ctx, pos.TierId); err != nil {
			return err
		}
		return nil
	}

	if oldPos.Owner != pos.Owner {
		oldOwner, err := sdk.AccAddressFromBech32(oldPos.Owner)
		if err != nil {
			return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
		}
		if err := k.PositionsByOwner.Remove(ctx, collections.Join(oldOwner, pos.Id)); err != nil {
			return err
		}
		newOwner, err := sdk.AccAddressFromBech32(pos.Owner)
		if err != nil {
			return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
		}
		if err := k.PositionsByOwner.Set(ctx, collections.Join(newOwner, pos.Id)); err != nil {
			return err
		}
	}

	if oldPos.TierId != pos.TierId {
		if err := k.PositionsByTier.Remove(ctx, collections.Join(oldPos.TierId, pos.Id)); err != nil {
			return err
		}
		if err := k.PositionsByTier.Set(ctx, collections.Join(pos.TierId, pos.Id)); err != nil {
			return err
		}
		if err := k.decreasePositionCount(ctx, oldPos.TierId); err != nil {
			return err
		}
		if err := k.increasePositionCount(ctx, pos.TierId); err != nil {
			return err
		}
	}

	oldDelegated := oldPos.IsDelegated()
	newDelegated := pos.IsDelegated()
	changedValidator := oldPos.Validator != pos.Validator
	if oldDelegated && (!newDelegated || changedValidator) {
		oldVal, err := sdk.ValAddressFromBech32(oldPos.Validator)
		if err != nil {
			return err
		}
		if err := k.PositionsByValidator.Remove(ctx, collections.Join(oldVal, pos.Id)); err != nil {
			return err
		}
	}
	if newDelegated && (!oldDelegated || changedValidator) {
		newVal, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return err
		}
		if err := k.PositionsByValidator.Set(ctx, collections.Join(newVal, pos.Id)); err != nil {
			return err
		}
	}
	return nil
}

// DeletePosition removes a position and cleans up secondary indexes.
func (k Keeper) deletePosition(ctx context.Context, pos types.Position) error {
	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	// guard, but should already be deleted by unbonding completion hook.
	if err := k.deleteUnbondingMappingsForPosition(ctx, pos.Id); err != nil {
		return err
	}

	// guard, but should already be deleted by redelegation completion hook,
	// unless a redelegation position gets slashed to zero after exit duration is elapsed and exits.
	if err := k.deleteRedelegationMappingsForPosition(ctx, pos.Id); err != nil {
		return err
	}

	if err := k.Positions.Remove(ctx, pos.Id); err != nil {
		return err
	}
	if err := k.PositionsByOwner.Remove(ctx, collections.Join(owner, pos.Id)); err != nil {
		return err
	}
	if err := k.PositionsByTier.Remove(ctx, collections.Join(pos.TierId, pos.Id)); err != nil {
		return err
	}
	if pos.IsDelegated() {
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return err
		}
		if err := k.PositionsByValidator.Remove(ctx, collections.Join(valAddr, pos.Id)); err != nil {
			return err
		}
	}
	return k.decreasePositionCount(ctx, pos.TierId)
}

func (k Keeper) getPositionsIdsByOwner(ctx context.Context, owner sdk.AccAddress) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[sdk.AccAddress, uint64](owner)
	return collectPairKeySetK2(ctx, k.PositionsByOwner, rng)
}

func (k Keeper) getPositionsIdsByValidator(ctx context.Context, valAddr sdk.ValAddress) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr)
	return collectPairKeySetK2(ctx, k.PositionsByValidator, rng)
}

func (k Keeper) getPositionsByIds(ctx context.Context, ids []uint64) ([]types.Position, error) {
	positions := make([]types.Position, 0, len(ids))
	for _, id := range ids {
		pos, err := k.getPosition(ctx, id)
		if errors.Is(err, types.ErrPositionNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, nil
}

func (k Keeper) GetPositionsByOwner(ctx context.Context, owner sdk.AccAddress) ([]types.Position, error) {
	ids, err := k.getPositionsIdsByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	return k.getPositionsByIds(ctx, ids)
}

func (k Keeper) getPositionsByValidator(ctx context.Context, valAddr sdk.ValAddress) ([]types.Position, error) {
	ids, err := k.getPositionsIdsByValidator(ctx, valAddr)
	if err != nil {
		return nil, err
	}
	return k.getPositionsByIds(ctx, ids)
}

func (k Keeper) getPositionsIdsByTier(ctx context.Context, tierId uint32) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[uint32, uint64](tierId)
	return collectPairKeySetK2(ctx, k.PositionsByTier, rng)
}

func (k Keeper) getPositionsByTier(ctx context.Context, tierId uint32) ([]types.Position, error) {
	ids, err := k.getPositionsIdsByTier(ctx, tierId)
	if err != nil {
		return nil, err
	}
	return k.getPositionsByIds(ctx, ids)
}
