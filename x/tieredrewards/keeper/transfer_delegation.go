package keeper

import (
	"context"
	"errors"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// TransferDelegation transfers delegation shares from a delegator to the
// tier module on the same validator. The delegator's tokens are unbonded and
// re-delegated from the module account to the same validator, transferring
// ownership without changing the validator.
//
// Only allow transfer to a bonded validator.
// Does not allow transfer if the delegator has an active incoming redelegation
// to the validator.
// Unbonding delegation is not an issue here because it would already been removed from the delegation
//
// Returns the new shares created for the module's delegation.
func (k Keeper) TransferDelegation(ctx context.Context, msg types.MsgCommitDelegationToTier) (math.LegacyDec, error) {
	if !msg.Amount.IsPositive() {
		return math.LegacyDec{}, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"delegation amount must be positive",
		)
	}

	from, err := sdk.AccAddressFromBech32(msg.DelegatorAddress)
	if err != nil {
		return math.LegacyDec{}, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid delegator address")
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return math.LegacyDec{}, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	if from.Equals(poolAddr) {
		return math.LegacyDec{}, types.ErrTransferDelegationToPoolSelf
	}

	validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		return math.LegacyDec{}, types.ErrBadTransferDelegationSrc
	} else if err != nil {
		return math.LegacyDec{}, err
	}

	// Only allow transferring delegations on bonded validators.
	// Non-bonded validators earn no rewards, so creating a tier position
	// for them would produce incorrect reward accounting.
	if !validator.IsBonded() {
		return math.LegacyDec{}, types.ErrValidatorNotBonded
	}

	// Block transfer if the delegator has an active incoming redelegation
	// to this validator. Without this check, the user could escape slashing
	// at the source validator by transferring the destination delegation to
	// the tier module — the redelegation entry would reference shares that
	// no longer belong to the user, causing the slash to miss.
	hasRedel, err := k.stakingKeeper.HasReceivingRedelegation(ctx, from, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}
	if hasRedel {
		return math.LegacyDec{}, types.ErrActiveRedelegation
	}

	shares, err := k.stakingKeeper.ValidateUnbondAmount(ctx, from, valAddr, msg.Amount)
	if err != nil {
		return math.LegacyDec{}, err
	}

	returnAmount, err := k.stakingKeeper.Unbond(ctx, from, valAddr, shares)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if returnAmount.IsZero() {
		return math.LegacyDec{}, types.ErrTinyTransferDelegationAmount
	}

	// Re-fetch validator after unbond since tokens and exchange rate changed.
	validator, err = k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}

	newShares, err := k.stakingKeeper.Delegate(ctx, poolAddr, returnAmount, validator.GetStatus(), validator, false)
	if err != nil {
		return math.LegacyDec{}, err
	}

	return newShares, nil
}
