package types

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg = &MsgLockTier{}
	_ sdk.Msg = &MsgCommitDelegationToTier{}
)

// Validate validates MsgLockTier sdk msg.
func (msg MsgLockTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	if msg.ValidatorAddress != "" {
		if _, err := sdk.ValAddressFromBech32(msg.ValidatorAddress); err != nil {
			return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
		}
	}

	return nil
}

// Validate validates MsgCommitDelegationToTier sdk msg.
func (msg MsgCommitDelegationToTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.DelegatorAddress); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid delegator address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	// Validator is required for commit delegation
	if _, err := sdk.ValAddressFromBech32(msg.ValidatorAddress); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	return nil
}
