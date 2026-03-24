package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
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
// Exiting positions are allowed to delegate per ADR-006 §10: a position
// created with trigger_exit_immediately but no validator can still be
// delegated later so it earns rewards until ExitUnlockTime.
func (k Keeper) ValidateDelegatePosition(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return types.ErrNotPositionOwner
	}

	if pos.IsDelegated() {
		return types.ErrPositionAlreadyDelegated
	}

	return nil
}

// ValidateUndelegatePosition validates the position intended to be undelegated.
// Per ADR-006 §5.4, undelegation is allowed as soon as the user has triggered
// exit (ExitTriggeredAt != 0). The user does not need to wait for the full
// exit commitment to elapse before beginning the SDK unbonding period.
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

// ValidateWithdrawFromTier validates a position is ready for token withdrawal.
// The position must have triggered exit, the exit commitment must have elapsed,
// and the position must not still be delegated.
func (k Keeper) ValidateWithdrawFromTier(ctx context.Context, pos types.Position, owner string) error {
	if pos.Owner != owner {
		return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "signer is not position owner")
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
