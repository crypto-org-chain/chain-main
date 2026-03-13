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

// TransferDelegationToPool transfers delegation shares from a delegator to the pool.
// It supports both same-validator transfers and cross-validator transfers.
//
// This is essentially a special case of redelegation,
// where apart from validator change, there can also be a change in shares ownership.
//
// For same-validator transfers (SrcValidator == DstValidator), the validator's
// tokens and shares are temporarily reduced by Unbond and restored by Delegate,
// resulting in a net-zero change to the validator.
//
// For cross-validator transfers, tokens are moved between validators and pool
// transfers are handled automatically based on validator bond statuses.
//
// Returns the new shares created for the pool.
func (k Keeper) TransferDelegationToPool(ctx context.Context, msg types.MsgCommitDelegationToTier) (math.LegacyDec, error) {
	if !msg.Amount.IsValid() {
		return math.LegacyDec{}, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"invalid delegation amount",
		)
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if msg.Amount.Denom != bondDenom {
		return math.LegacyDec{}, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "invalid coin denomination: got %s, expected %s", msg.Amount.Denom, bondDenom,
		)
	}

	from, err := sdk.AccAddressFromBech32(msg.DelegatorAddress)
	if err != nil {
		return math.LegacyDec{}, sdkerrors.ErrInvalidAddress
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	if from.Equals(poolAddr) {
		return math.LegacyDec{}, types.ErrTransferDelegationToPoolSelf
	}

	srcValidatorAddr, err := sdk.ValAddressFromBech32(msg.ValidatorSrcAddress)
	if err != nil {
		return math.LegacyDec{}, sdkerrors.ErrInvalidAddress
	}

	destValidatorAddr, err := sdk.ValAddressFromBech32(msg.ValidatorDstAddress)
	if err != nil {
		return math.LegacyDec{}, sdkerrors.ErrInvalidAddress
	}

	srcValidator, err := k.stakingKeeper.GetValidator(ctx, srcValidatorAddr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		return math.LegacyDec{}, types.ErrBadTransferDelegationSrc
	} else if err != nil {
		return math.LegacyDec{}, err
	}

	shares, err := k.stakingKeeper.ValidateUnbondAmount(
		ctx, from, srcValidatorAddr, msg.Amount.Amount,
	)

	if err != nil {
		return math.LegacyDec{}, err
	}

	returnAmount, err := k.stakingKeeper.Unbond(ctx, from, srcValidatorAddr, shares)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if returnAmount.IsZero() {
		return math.LegacyDec{}, types.ErrTinyTransferDelegationAmount
	}

	destValidator, err := k.stakingKeeper.GetValidator(ctx, destValidatorAddr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		return math.LegacyDec{}, types.ErrBadTransferDelegationDest
	} else if err != nil {
		return math.LegacyDec{}, err
	}

	newShares, err := k.stakingKeeper.Delegate(ctx, poolAddr, returnAmount, srcValidator.GetStatus(), destValidator, false)
	if err != nil {
		return math.LegacyDec{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventTransferDelegationToPool{
		TransferDelegation: msg,
	}); err != nil {
		return math.LegacyDec{}, err
	}

	return newShares, nil
}
