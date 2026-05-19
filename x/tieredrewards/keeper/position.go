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

// ValidatorTransition signals a change in the position's associated validator
// so that PositionCountByValidator can be reindexed.
type ValidatorTransition struct {
	PreviousAddress string
}

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

func (k Keeper) createDelegatedPosition(
	ctx context.Context,
	owner string,
	tier types.Tier,
	valAddr sdk.ValAddress,
	triggerExitImmediately bool,
) (types.Position, error) {
	id, err := k.NextPositionId.Next(ctx)
	if err != nil {
		return types.Position{}, err
	}

	lastEventSeq, err := k.getValidatorEventLatestSeq(ctx, valAddr)
	if err != nil {
		return types.Position{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()
	blockHeight := uint64(sdkCtx.BlockHeight())

	pos := types.NewPosition(id, owner, tier.Id, blockHeight, lastEventSeq, blockTime, true, blockTime)

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

// setPosition validates and persists pos and reconciles the relevant indexes.
func (k Keeper) setPosition(ctx context.Context, pos types.Position, update *ValidatorTransition) error {
	del, err := k.getDelegation(ctx, pos.Id)
	if err != nil {
		return err
	}
	return k.setPositionWithState(ctx, types.PositionState{Position: pos, Delegation: del}, update)
}

// setPositionWithState validates and persists the position with a supplied PositionState.
func (k Keeper) setPositionWithState(ctx context.Context, state types.PositionState, update *ValidatorTransition) error {
	if err := state.Validate(); err != nil {
		return err
	}

	pos := state.Position

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
		if err := k.increasePositionCountForTier(ctx, pos.TierId); err != nil {
			return err
		}
	} else {
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
	}

	if update == nil {
		return nil
	}
	currVal := ""
	if state.IsDelegated() {
		currVal = state.Delegation.ValidatorAddress
	}
	return k.reindexPositionCountByValidator(ctx, update.PreviousAddress, currVal)
}

// deletePosition validates and removes a position and cleans up secondary indexes.
func (k Keeper) deletePosition(ctx context.Context, pos types.Position, update *ValidatorTransition) error {
	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	del, err := k.getDelegation(ctx, pos.Id)
	if err != nil {
		return err
	}
	if del != nil {
		return errorsmod.Wrapf(types.ErrPositionDelegated, "cannot delete position %d: still has active delegation to %s", pos.Id, del.ValidatorAddress)
	}

	delAddr := types.GetDelegatorAddress(pos.Id)

	if err := k.removeBaseRewardsRouting(ctx, delAddr, owner); err != nil {
		return err
	}

	// Defensive
	if err := k.deletePositionRedelegationMappings(ctx, pos.Id); err != nil {
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
	if err := k.decreasePositionCountForTier(ctx, pos.TierId); err != nil {
		return err
	}

	if update == nil {
		return nil
	}
	return k.reindexPositionCountByValidator(ctx, update.PreviousAddress, "")
}

func (k Keeper) reindexPositionCountByValidator(ctx context.Context, from, to string) error {
	if from == to {
		return nil
	}
	if from != "" {
		valAddr, err := sdk.ValAddressFromBech32(from)
		if err != nil {
			return err
		}
		if err := k.decreasePositionCountForValidator(ctx, valAddr); err != nil {
			return err
		}
	}
	if to != "" {
		valAddr, err := sdk.ValAddressFromBech32(to)
		if err != nil {
			return err
		}
		if err := k.increasePositionCountForValidator(ctx, valAddr); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) getPositionsIdsByOwner(ctx context.Context, owner sdk.AccAddress) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[sdk.AccAddress, uint64](owner)
	return collectPairKeySetK2(ctx, k.PositionsByOwner, rng)
}

func (k Keeper) getPositionsIdsByTier(ctx context.Context, tierId uint32) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[uint32, uint64](tierId)
	return collectPairKeySetK2(ctx, k.PositionsByTier, rng)
}
