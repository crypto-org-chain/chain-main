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
)

func (k Keeper) getPosition(ctx context.Context, id uint64) (types.Position, error) {
	pos, err := k.Positions.Get(ctx, id)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.Position{}, errorsmod.Wrapf(types.ErrPositionNotFound, "position id %d", id)
		}
		return types.Position{}, errorsmod.Wrapf(err, "%s (position id %d)", types.ErrPositionStore.Error(), id)
	}
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

	delAddr := types.GetDelegatorAddress(id)

	ownerAddr, err := sdk.AccAddressFromBech32(owner)
	if err != nil {
		return types.Position{}, err
	}

	if err := k.routeBaseRewardsToOwner(ctx, delAddr, ownerAddr); err != nil {
		return types.Position{}, err
	}

	if triggerExitImmediately {
		pos.TriggerExit(blockTime, tier.ExitDuration)
	}

	if err := k.setPosition(ctx, pos); err != nil {
		return types.Position{}, err
	}

	return pos, nil
}

// lockFunds locks the desired amount of funds into a position.
func (k Keeper) lockFunds(ctx context.Context, ownerAddr, delAddr sdk.AccAddress, amount math.Int) error {
	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}
	return k.bankKeeper.SendCoins(ctx, ownerAddr, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, amount)))
}

// createDelegatorAccount creates a BaseAccount for the position's delegation.
// This is required so that undelegation can complete successfully because it checks for the existence of the account in bank keeper trackUndelegation method.
func (k Keeper) createDelegatorAccount(ctx context.Context, delAddr sdk.AccAddress) {
	if k.accountKeeper.GetAccount(ctx, delAddr) != nil {
		return
	}
	acc := k.accountKeeper.NewAccountWithAddress(ctx, delAddr)
	k.accountKeeper.SetAccount(ctx, acc)
}

// routeBaseRewardsToOwner routes base rewards for the position's delegation directly to the position owner.
func (k Keeper) routeBaseRewardsToOwner(ctx context.Context, posDelAddr, ownerAddr sdk.AccAddress) error {
	return k.distributionKeeper.SetWithdrawAddr(ctx, posDelAddr, ownerAddr)
}

// removeBaseRewardsRouting removes the routing of base rewards for the position's delegation to the position owner.
// Part of position clean up during deletion.
func (k Keeper) removeBaseRewardsRouting(ctx context.Context, posDelAddr, ownerAddr sdk.AccAddress) error {
	return k.distributionKeeper.DeleteDelegatorWithdrawAddr(ctx, posDelAddr, ownerAddr)
}

// SetPosition stores a position, validates it, and maintains secondary indexes.
func (k Keeper) setPosition(ctx context.Context, pos types.Position) error {
	if err := pos.Validate(); err != nil {
		return err
	}
	oldPos, err := k.getPosition(ctx, pos.Id)
	isNew := errors.Is(err, types.ErrPositionNotFound)

	if !isNew && err != nil {
		return err
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
			if err := k.increasePositionCountForValidator(ctx, valAddr); err != nil {
				return err
			}
		}
		if err := k.increasePositionCountForTier(ctx, pos.TierId); err != nil {
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
		if err := k.decreasePositionCountForTier(ctx, oldPos.TierId); err != nil {
			return err
		}
		if err := k.increasePositionCountForTier(ctx, pos.TierId); err != nil {
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
		if err := k.decreasePositionCountForValidator(ctx, oldVal); err != nil {
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
		if err := k.increasePositionCountForValidator(ctx, newVal); err != nil {
			return err
		}
	}
	return nil
}

// setPositionUnsafe persists a position without reading the old value or diffing
// secondary indexes. Use only when the caller guarantees that owner, tier, and
// validator have not changed (e.g., after claiming rewards).
func (k Keeper) setPositionUnsafe(ctx context.Context, pos types.Position) error {
	return k.Positions.Set(ctx, pos.Id, pos)
}

// DeletePosition removes a position and cleans up secondary indexes.
func (k Keeper) deletePosition(ctx context.Context, pos types.Position) error {
	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	delAddr := types.GetDelegatorAddress(pos.Id)

	if err := k.removeBaseRewardsRouting(ctx, delAddr, owner); err != nil {
		return err
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
		if err := k.decreasePositionCountForValidator(ctx, valAddr); err != nil {
			return err
		}
	}
	return k.decreasePositionCountForTier(ctx, pos.TierId)
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
