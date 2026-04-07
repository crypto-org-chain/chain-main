package types

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg = &MsgLockTier{}
	_ sdk.Msg = &MsgCommitDelegationToTier{}
	_ sdk.Msg = &MsgTierDelegate{}
	_ sdk.Msg = &MsgTierUndelegate{}
	_ sdk.Msg = &MsgTierRedelegate{}
	_ sdk.Msg = &MsgAddToTierPosition{}
	_ sdk.Msg = &MsgTriggerExitFromTier{}
	_ sdk.Msg = &MsgClearPosition{}
	_ sdk.Msg = &MsgClaimTierRewards{}
	_ sdk.Msg = &MsgWithdrawFromTier{}
)

func (msg MsgLockTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	if _, err := sdk.ValAddressFromBech32(msg.ValidatorAddress); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	return nil
}

func (msg MsgCommitDelegationToTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.DelegatorAddress); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid delegator address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	if _, err := sdk.ValAddressFromBech32(msg.ValidatorAddress); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	return nil
}

func (msg MsgTierDelegate) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if _, err := sdk.ValAddressFromBech32(msg.Validator); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	return nil
}

func (msg MsgTierUndelegate) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

func (msg MsgTierRedelegate) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if _, err := sdk.ValAddressFromBech32(msg.DstValidator); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid destination validator address")
	}

	return nil
}

func (msg MsgAddToTierPosition) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	return nil
}

func (msg MsgTriggerExitFromTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

func (msg MsgClearPosition) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

func (msg MsgClaimTierRewards) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

func (msg MsgWithdrawFromTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

