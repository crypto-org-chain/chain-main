package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// transferDelegationToPosition transfers delegation shares from
// the original owner to a position's delegation address on the same validator.
// Shares are unbonded at the source and re-delegated from the destination
// with no unbonding period.
//
// Only bonded validators are allowed. Blocks transfer if the delegator has an
// active incoming redelegation to the validator.
func (k Keeper) transferDelegationToPosition(ctx context.Context, owner string, posDelAddr sdk.AccAddress, validatorAddr string, amount math.Int) (math.LegacyDec, error) {
	if !amount.IsPositive() {
		return math.LegacyDec{}, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"delegation amount must be positive",
		)
	}

	from, err := sdk.AccAddressFromBech32(owner)
	if err != nil {
		return math.LegacyDec{}, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid delegator address")
	}

	valAddr, err := sdk.ValAddressFromBech32(validatorAddr)
	if err != nil {
		return math.LegacyDec{}, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	if from.Equals(posDelAddr) {
		return math.LegacyDec{}, types.ErrTransferDelegationToPositionSelf
	}

	validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		return math.LegacyDec{}, types.ErrTransferDelegationSrcNotFound
	} else if err != nil {
		return math.LegacyDec{}, err
	}

	if !validator.IsBonded() {
		return math.LegacyDec{}, types.ErrValidatorNotBonded
	}

	// Block transfer with active incoming redelegation: user could escape
	// slashing at the source validator by moving the destination delegation
	// to the tier module.
	hasRedel, err := k.stakingKeeper.HasReceivingRedelegation(ctx, from, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}
	if hasRedel {
		return math.LegacyDec{}, types.ErrActiveRedelegation
	}

	shares, err := k.stakingKeeper.ValidateUnbondAmount(ctx, from, valAddr, amount)
	if err != nil {
		return math.LegacyDec{}, err
	}

	newAmount, err := k.stakingKeeper.Unbond(ctx, from, valAddr, shares)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if newAmount.IsZero() {
		return math.LegacyDec{}, types.ErrTinyTransferDelegationAmount
	}

	// Re-fetch validator after unbond since tokens and exchange rate changed.
	validator, err = k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}

	newShares, err := k.stakingKeeper.Delegate(ctx, posDelAddr, newAmount, validator.GetStatus(), validator, false)
	if err != nil {
		return math.LegacyDec{}, err
	}

	return newShares, nil
}

// transferDelegationFromPosition transfers delegation shares from the position's
// delegator address back to the owner on the same validator. The position's
// delegation is unbonded and re-delegated from the owner's address. No
// unbonding period.
func (k Keeper) transferDelegationFromPosition(ctx context.Context, pos types.Position, valAddr sdk.ValAddress, amount math.Int) (math.LegacyDec, math.LegacyDec, math.Int, error) {
	owner, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Defensive
	if !pos.IsDelegated() {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, errorsmod.Wrapf(types.ErrPositionNotDelegated, "position %d is not delegated", pos.Id)
	}

	// Defensive
	redelegating, err := k.stillRedelegating(ctx, pos.Id)
	if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
	}
	if redelegating {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, errorsmod.Wrapf(types.ErrActiveRedelegation, "position %d has an active redelegation", pos.Id)
	}

	validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, types.ErrTransferDelegationDestNotFound
	} else if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
	}

	if !validator.IsBonded() {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, types.ErrValidatorNotBonded
	}

	tokenValue, err := k.reconcileAmountFromShares(ctx, valAddr, pos.DelegatedShares)
	if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
	}

	unbondedShares := pos.DelegatedShares
	if !pos.ExitWithFullDelegation(amount, tokenValue) {
		unbondedShares, err = k.stakingKeeper.ValidateUnbondAmount(ctx, posDelAddr, valAddr, amount)
		if err != nil {
			return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
		}
	}

	transferredAmount, err := k.stakingKeeper.Unbond(ctx, posDelAddr, valAddr, unbondedShares)
	if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
	}

	if transferredAmount.IsZero() {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, types.ErrTinyTransferDelegationAmount
	}

	// Re-fetch validator after unbond since tokens and exchange rate changed.
	validator, err = k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
	}

	ownerNewShares, err := k.stakingKeeper.Delegate(ctx, owner, transferredAmount, validator.GetStatus(), validator, false)
	if err != nil {
		return math.LegacyDec{}, math.LegacyDec{}, math.Int{}, err
	}

	return ownerNewShares, unbondedShares, transferredAmount, nil
}
