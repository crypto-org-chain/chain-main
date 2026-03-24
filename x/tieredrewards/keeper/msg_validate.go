package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) ValidateNewPosition(ctx context.Context, tier types.Tier, amount math.Int) error {
	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	if !tier.MeetsMinLockRequirement(amount) {
		return types.ErrMinLockAmountNotMet
	}

	return nil
}

func (k Keeper) ValidateDelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.IsDelegated() {
		return types.ErrPositionAlreadyDelegated
	}

	return nil
}

func (k Keeper) ValidateUndelegatePosition(ctx context.Context, pos types.Position, owner string) error {
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

func (k Keeper) ValidateRedelegatePosition(ctx context.Context, pos types.Position, owner, dstValidator string) error {
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

func (k Keeper) ValidateAddToPosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
	}

	tier, err := k.GetTier(ctx, pos.TierId)
	if err != nil {
		return err
	}

	if tier.IsCloseOnly() {
		return types.ErrTierIsCloseOnly
	}

	return nil
}

func (k Keeper) ValidateTriggerExit(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
	}

	return nil
}

func (k Keeper) ValidateClaimRewards(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	return nil
}

func (k Keeper) ValidateWithdrawFromTier(ctx context.Context, pos types.Position, owner string) error {
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
