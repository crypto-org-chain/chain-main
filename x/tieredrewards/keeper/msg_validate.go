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
//
// NOTE: After undelegation, the SDK unbonding period runs in the
// background. If the user re-delegates this position to a new validator
// before the previous unbonding completes, a slash on the old validator
// could still reduce the position's DelegatedShares via
// AfterSlashUnbondingDelegation while the position is already earning
// on the new validator. The UnbondingIdToPositionId mapping tracks
// in-flight unbondings for this reason; callers should be aware that
// the position may carry residual slash exposure from a prior validator.
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
// Exiting positions are blocked from redelegation because the owner has
// committed to leaving the tier; only undelegation is allowed after exit.
//
// NOTE: A redelegation creates a transient unbonding entry on the
// source validator (tracked via UnbondingIdToPositionId). If the source
// validator is slashed during the redelegation maturation window, the
// SDK routes the slash through AfterSlashRedelegation, which adjusts
// the position's DelegatedShares on the destination validator. A second
// redelegation from the destination to a third validator before the
// first redelegation matures would create overlapping slash exposure;
// the SDK's own redelegation-hop restriction prevents this at the
// staking layer (BeginRedelegation rejects if destination has a pending
// incoming redelegation from the same source).
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
