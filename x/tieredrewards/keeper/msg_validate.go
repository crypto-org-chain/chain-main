package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) validateNewPosition(tier types.Tier, amount math.Int) error {
	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	if !tier.MeetsMinLockRequirement(amount) {
		return types.ErrMinLockAmountNotMet
	}

	return nil
}

func (k Keeper) validateUndelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	if !pos.HasTriggeredExit() {
		return types.ErrExitNotTriggered
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if !pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		return types.ErrExitLockDurationNotReached
	}

	// skip check for zero amount as we want those positions to be able to close their position properly

	return nil
}

func (k Keeper) validateRedelegatePosition(ctx context.Context, pos types.Position, owner, dstValidator string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	if !pos.DelegatedShares.IsPositive() {
		return types.ErrPositionSharesZero
	}

	if pos.Validator == dstValidator {
		return types.ErrRedelegationToSameValidator
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if pos.HasTriggeredExit() && pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		return types.ErrExitLockDurationElapsed
	}

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return err
	}

	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	return nil
}

func (k Keeper) validateAddToPosition(ctx context.Context, pos types.Position, owner string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionTriggeredExit
	}

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return err
	}

	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	return nil
}

func (k Keeper) validateTriggerExit(pos types.Position, owner string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionTriggeredExit
	}

	return nil
}

func (k Keeper) validateClearPosition(ctx context.Context, pos types.Position, owner string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		unbonding, err := k.stillUnbonding(ctx, pos.Id)
		if err != nil {
			return err
		}
		if unbonding {
			return types.ErrPositionUnbonding
		}

		if !pos.IsDelegated() {
			return types.ErrPositionNotDelegated
		}

	}

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return err
	}

	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	return nil
}

func (k Keeper) validateClaimRewards(pos types.Position, owner string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	return nil
}

func (k Keeper) validateWithdrawFromTier(ctx context.Context, pos types.Position, owner string) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	if !pos.HasTriggeredExit() {
		return types.ErrExitNotTriggered
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if !pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		return types.ErrExitLockDurationNotReached
	}

	if pos.IsDelegated() {
		return types.ErrPositionDelegated
	}

	unbonding, err := k.stillUnbonding(ctx, pos.Id)
	if err != nil {
		return err
	}
	if unbonding {
		return types.ErrPositionUnbonding
	}
	// no need to check for redelegation because unbonding only allowed after exit duration elapsed
	// and redelegation is not allowed after exit duration elapsed
	// Therefore, any redelegation would have matured by the time any unbonding matures (same unbonding duration)

	return nil
}

func (k Keeper) validateExitTierWithDelegation(ctx context.Context, pos types.Position, owner string, amount math.Int) error {
	if !pos.IsOwner(owner) {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	if !pos.HasTriggeredExit() {
		return types.ErrExitNotTriggered
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if !pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		return types.ErrExitLockDurationNotReached
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return err
	}
	tokenValue, err := k.reconcileAmountFromShares(ctx, valAddr, pos.DelegatedShares)
	if err != nil {
		return err
	}

	if amount.GT(tokenValue) {
		return errorsmod.Wrapf(types.ErrInvalidAmount, "amount %s exceeds position token value %s", amount, tokenValue)
	}

	redelegating, err := k.stillRedelegating(ctx, pos.Id)
	if err != nil {
		return err
	}
	if redelegating {
		return errorsmod.Wrapf(types.ErrActiveRedelegation, "position %d has an active redelegation", pos.Id)
	}

	return nil
}
