package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// CreatePosition locks tokens from the owner into the tier module and creates a new position.
// If a validator address is provided, the locked tokens are immediately delegated to that validator.
// If triggerExitImmediately is true, the exit commitment starts from lock time.
// The tier must exist, not be in close-only mode, and the amount must meet the tier's minimum lock requirement.
func (k Keeper) CreatePosition(
	ctx context.Context,
	owner string,
	tierId uint32,
	amount math.Int,
	validator string,
	triggerExitImmediately bool) (types.Position, error) {

	tier, err := k.Tiers.Get(ctx, tierId)
	if err != nil {
		return types.Position{}, err
	}

	if tier.IsCloseOnly() {
		return types.Position{}, types.ErrTierIsCloseOnly
	}

	if !tier.MeetsMinLockRequirement(amount) {
		return types.Position{}, types.ErrMinLockAmountNotMet
	}

	ownerAddr, err := sdk.AccAddressFromBech32(owner)
	if err != nil {
		return types.Position{}, sdkerrors.ErrInvalidAddress
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return types.Position{}, err
	}

	err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, types.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, amount)))
	if err != nil {
		return types.Position{}, err
	}

	id, err := k.NextPositionId.Next(ctx)
	if err != nil {
		return types.Position{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	pos := types.NewPosition(id, owner, tierId, amount, sdkCtx.BlockHeight(), sdkCtx.BlockTime())

	if validator != "" {
		shares, err := k.delegateFromPosition(ctx, validator, amount)
		if err != nil {
			return types.Position{}, err
		}
		pos.UpdateDelegation(validator, shares)
	}

	if triggerExitImmediately {
		pos.TriggerExit(sdkCtx.BlockTime(), tier.ExitDuration)
	}

	return k.SetPosition(ctx, pos)
}

// SetPosition stores a position. Validates and maintains secondary indexes.
func (k Keeper) SetPosition(ctx context.Context, pos types.Position) (types.Position, error) {
	if err := pos.Validate(); err != nil {
		return types.Position{}, err
	}
	oldPos, err := k.Positions.Get(ctx, pos.Id)
	isNew := errors.Is(err, collections.ErrNotFound)

	if !isNew && err != nil {
		return types.Position{}, err
	}

	if err == nil && oldPos.IsDelegated() && oldPos.Validator != pos.Validator {
		oldVal, _ := sdk.ValAddressFromBech32(oldPos.Validator)
		if oldVal != nil {
			_ = k.PositionsByValidator.Remove(ctx, collections.Join(oldVal, pos.Id))
		}
	}

	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return types.Position{}, sdkerrors.ErrInvalidAddress
	}

	if err := k.Positions.Set(ctx, pos.Id, pos); err != nil {
		return types.Position{}, err
	}

	if err := k.PositionsByOwner.Set(ctx, collections.Join(owner, pos.Id)); err != nil {
		return types.Position{}, err
	}

	if err := k.PositionsByTier.Set(ctx, collections.Join(pos.TierId, pos.Id)); err != nil {
		return types.Position{}, err
	}

	if pos.IsDelegated() {
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return types.Position{}, err
		}
		if err := k.PositionsByValidator.Set(ctx, collections.Join(valAddr, pos.Id)); err != nil {
			return types.Position{}, err
		}
	}

	if isNew {
		err = k.increasePositionCount(ctx, pos.TierId)
		if err != nil {
			return types.Position{}, err
		}
	}
	return pos, nil
}

// DeletePosition removes a position and its secondary indexes.
func (k Keeper) DeletePosition(ctx context.Context, pos types.Position) error {
	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
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

// GetPositionsIdsByOwner returns all position IDs owned by an address.
func (k Keeper) GetPositionsIdsByOwner(ctx context.Context, owner sdk.AccAddress) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[sdk.AccAddress, uint64](owner)
	return types.CollectPairKeySetK2(ctx, k.PositionsByOwner, rng)
}

// GetPositionsIdsByValidator returns all position IDs delegated to a validator.
func (k Keeper) GetPositionsIdsByValidator(ctx context.Context, valAddr sdk.ValAddress) ([]uint64, error) {
	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr)
	return types.CollectPairKeySetK2(ctx, k.PositionsByValidator, rng)
}

// GetPositionsByIds returns positions for the given IDs.
func (k Keeper) GetPositionsByIds(ctx context.Context, ids []uint64) ([]types.Position, error) {
	positions := make([]types.Position, 0, len(ids))
	for _, id := range ids {
		pos, err := k.Positions.Get(ctx, id)
		if errors.Is(err, collections.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, nil
}

// GetPositionsByOwner returns all positions owned by an address.
func (k Keeper) GetPositionsByOwner(ctx context.Context, owner sdk.AccAddress) ([]types.Position, error) {
	ids, err := k.GetPositionsIdsByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	return k.GetPositionsByIds(ctx, ids)
}

// GetPositionsByValidator returns all positions delegated to a validator.
func (k Keeper) GetPositionsByValidator(ctx context.Context, valAddr sdk.ValAddress) ([]types.Position, error) {
	ids, err := k.GetPositionsIdsByValidator(ctx, valAddr)
	if err != nil {
		return nil, err
	}
	return k.GetPositionsByIds(ctx, ids)
}
