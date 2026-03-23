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
	_ sdk.Msg = &MsgClaimTierRewards{}
	_ sdk.Msg = &MsgWithdrawFromTier{}
	_ sdk.Msg = &MsgFundTierPool{}
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

// Validate validates MsgTierDelegate sdk msg.
func (msg MsgTierDelegate) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if _, err := sdk.ValAddressFromBech32(msg.Validator); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid validator address")
	}

	return nil
}

// Validate validates MsgTierUndelegate sdk msg.
func (msg MsgTierUndelegate) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

// Validate validates MsgTierRedelegate sdk msg.
func (msg MsgTierRedelegate) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if _, err := sdk.ValAddressFromBech32(msg.DstValidator); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid destination validator address")
	}

	return nil
}

// Validate validates MsgAddToTierPosition sdk msg.
func (msg MsgAddToTierPosition) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	return nil
}

// Validate validates MsgTriggerExitFromTier sdk msg.
func (msg MsgTriggerExitFromTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

// Validate validates MsgClaimTierRewards sdk msg.
func (msg MsgClaimTierRewards) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

// Validate validates MsgWithdrawFromTier sdk msg.
func (msg MsgWithdrawFromTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

// Validate validates MsgFundTierPool sdk msg.
func (msg MsgFundTierPool) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Depositor); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid depositor address")
	}

	if !msg.Amount.IsValid() || msg.Amount.IsZero() {
		return errorsmod.Wrap(ErrInvalidAmount, "amount must be valid and non-zero")
	}

	return nil
}
