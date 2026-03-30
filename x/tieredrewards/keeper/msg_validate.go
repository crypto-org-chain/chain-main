package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) validateNewPosition(ctx context.Context, tier types.Tier, amount math.Int) error {
	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	if !tier.MeetsMinLockRequirement(amount) {
		return types.ErrMinLockAmountNotMet
	}

	return nil
}

func (k Keeper) validateDelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.IsDelegated() {
		return types.ErrPositionAlreadyDelegated
	}

	return nil
}

func (k Keeper) validateUndelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	if !pos.HasTriggeredExit() {
		return types.ErrExitNotTriggered
	}

	return nil
}

func (k Keeper) validateRedelegatePosition(ctx context.Context, pos types.Position, owner, dstValidator string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
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

func (k Keeper) validateAddToPosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
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

func (k Keeper) validateTriggerExit(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
	}

	return nil
}

func (k Keeper) validateClearPosition(_ context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	return nil
}

func (k Keeper) validateClaimRewards(pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	return nil
}

func (k Keeper) validateWithdrawFromTier(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if !pos.HasTriggeredExit() {
		return types.ErrPositionNotReadyToWithdraw
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if !pos.CompletedExitLockDuration(sdkCtx.BlockTime()) {
		return types.ErrExitLockDurationNotReached
	}

	if pos.IsDelegated() {
		return types.ErrPositionStillDelegated
	}

	return nil
}
