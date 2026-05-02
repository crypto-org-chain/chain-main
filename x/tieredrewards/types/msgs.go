package types

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MaxClaimPositionIds is the maximum number of position IDs that can be
// claimed in a single MsgClaimTierRewards transaction.
const MaxClaimPositionIds = 300

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
	_ sdk.Msg = &MsgExitTierWithDelegation{}
)

func (msg MsgLockTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if msg.Id == 0 {
		return ErrInvalidTierID
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

	if msg.Id == 0 {
		return ErrInvalidTierID
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

	if len(msg.PositionIds) == 0 {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "position_ids must not be empty")
	}

	if len(msg.PositionIds) > MaxClaimPositionIds {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "too many position_ids: %d, max: %d", len(msg.PositionIds), MaxClaimPositionIds)
	}

	seen := make(map[uint64]struct{}, len(msg.PositionIds))
	for _, id := range msg.PositionIds {
		if _, dup := seen[id]; dup {
			return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "duplicate position_id %d", id)
		}
		seen[id] = struct{}{}
	}

	return nil
}

func (msg MsgWithdrawFromTier) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	return nil
}

func (msg MsgExitTierWithDelegation) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid owner address")
	}

	if !msg.Amount.IsPositive() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	return nil
}
