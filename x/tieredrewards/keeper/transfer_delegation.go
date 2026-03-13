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
		return math.LegacyDec{}, sdkerrors.ErrInvalidRequest
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return math.LegacyDec{}, sdkerrors.ErrInvalidRequest
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
