package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// ValidateNewPosition validates the new position creation.
func (k Keeper) ValidateNewPosition(ctx context.Context, tier types.Tier, amount math.Int) error {
	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	if !tier.MeetsMinLockRequirement(amount) {
		return types.ErrMinLockAmountNotMet
	}

	return nil
}

// ValidateDelegatePosition validates the position intended to be delegated.
func (k Keeper) ValidateDelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "signer is not position owner")
	}

	if pos.IsDelegated() {
		return types.ErrPositionAlreadyDelegated
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
	}

	return nil
}

// ValidateUndelegatePosition validates the position intended to be undelegated.
func (k Keeper) ValidateUndelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "signer is not position owner")
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if !pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		return types.ErrExitLockDurationNotReached
	}

	return nil
}

// ValidateRedelegatePosition validates the position intended to be redelegated.
func (k Keeper) ValidateRedelegatePosition(ctx context.Context, pos types.Position, owner string, dstValidator string) error {
	if pos.Owner != owner {
		return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "signer is not position owner")
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	if pos.Validator == dstValidator {
		return types.ErrRedelegationToSameValidator
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
	}

	return nil
}

func (k Keeper) CreatePosition(
	ctx context.Context,
	owner string,
	tier types.Tier,
	amount math.Int,
	delegation *types.Delegation,
	triggerExitImmediately bool,
) (types.Position, error) {
	id, err := k.NextPositionId.Next(ctx)
	if err != nil {
		return types.Position{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()
	blockHeight := sdkCtx.BlockHeight()

	pos := types.NewBasePosition(id, owner, tier.Id, amount, blockHeight, blockTime)

	if delegation != nil {
		pos.WithDelegation(*delegation, blockTime)
	}

	if triggerExitImmediately {
		pos.TriggerExit(blockTime, tier.ExitDuration)
	}

	if err := k.SetPosition(ctx, pos); err != nil {
		return types.Position{}, err
	}

	return pos, nil
}

// LockFunds locks the the desired amount of funds into a position.
func (k Keeper) LockFunds(ctx context.Context, owner string, amount math.Int) error {
	ownerAddr, err := sdk.AccAddressFromBech32(owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}

	err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, types.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, amount)))
	if err != nil {
		return err
	}
	return nil
}

// SetPosition stores a position. Validates and maintains secondary indexes.
func (k Keeper) SetPosition(ctx context.Context, pos types.Position) error {
	if err := pos.Validate(); err != nil {
		return err
	}
	oldPos, err := k.Positions.Get(ctx, pos.Id)
	isNew := errors.Is(err, collections.ErrNotFound)

	if !isNew && err != nil {
		return err
	}

	if err == nil && oldPos.IsDelegated() && oldPos.Validator != pos.Validator {
		oldVal, _ := sdk.ValAddressFromBech32(oldPos.Validator)
		if oldVal != nil {
			_ = k.PositionsByValidator.Remove(ctx, collections.Join(oldVal, pos.Id))
		}
	}

	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if err := k.Positions.Set(ctx, pos.Id, pos); err != nil {
		return err
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

	if isNew {
		err = k.increasePositionCount(ctx, pos.TierId)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeletePosition removes a position and its secondary indexes.
func (k Keeper) DeletePosition(ctx context.Context, pos types.Position) error {
	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
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
