package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
		return types.ErrNotPositionOwner
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
		return types.ErrNotPositionOwner
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

// ValidateAddToPosition validates the position intended to have tokens added.
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

// ValidateTriggerExit validates the position intended to trigger exit.
func (k Keeper) ValidateTriggerExit(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.HasTriggeredExit() {
		return types.ErrPositionExiting
	}

	return nil
}

// ValidateClaimRewards validates the position intended to claim rewards.
func (k Keeper) ValidateClaimRewards(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if !pos.IsDelegated() {
		return types.ErrPositionNotDelegated
	}

	return nil
}
