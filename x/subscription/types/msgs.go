package types

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Subscription message types and routes
const (
	TypeMsgCreatePlan         = "create_plan"
	TypeMsgStopPlan           = "stop_plan"
	TypeMsgCreateSubscription = "create_subscription"
	TypeMsgStopSubscription   = "stop_subscription"
)

// Constants pertaining to a contents
const (
	MaxDescriptionLength int = 5000
	MaxTitleLength       int = 140
)

var (
	_, _, _, _, _ sdk.Msg = &MsgCreatePlan{}, &MsgStopPlan{}, &MsgCreateSubscription{}, &MsgStopSubscription{}, &MsgStopUserSubscription{}
)

// GetSignBytes implements Msg
func (m MsgCreatePlan) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&m)
	return sdk.MustSortJSON(bz)
}

// Route implements Msg
func (m MsgCreatePlan) Route() string { return RouterKey }

// Type implements Msg
func (m MsgCreatePlan) Type() string { return TypeMsgCreatePlan }

// ValidateBasic implements Msg
func (m MsgCreatePlan) ValidateBasic() error {
	if m.Owner == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, m.Owner)
	}
	if !m.Price.IsValid() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, m.Price.String())
	}
	if m.DurationSecs == 0 {
		return ErrInvalidSubscriptionDuration
	}
	if !m.CronSpec.Compile().IsValid() {
		return sdkerrors.Wrap(ErrInvalidCronSpec, m.CronSpec.String())
	}
	if len(strings.TrimSpace(m.Title)) == 0 {
		return sdkerrors.Wrap(ErrInvalidPlanContent, "plan title cannot be blank")
	}
	if len(m.Title) > MaxTitleLength {
		return sdkerrors.Wrapf(ErrInvalidPlanContent, "plan title is longer than max length of %d", MaxTitleLength)
	}

	if len(m.Description) == 0 {
		return sdkerrors.Wrap(ErrInvalidPlanContent, "plan description cannot be blank")
	}
	if len(m.Description) > MaxDescriptionLength {
		return sdkerrors.Wrapf(ErrInvalidPlanContent, "plan description is longer than max length of %d", MaxDescriptionLength)
	}
	return nil
}

// GetSignBytes implements Msg
func (m MsgStopPlan) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&m)
	return sdk.MustSortJSON(bz)
}

// Route implements Msg
func (m MsgStopPlan) Route() string { return RouterKey }

// Type implements Msg
func (m MsgStopPlan) Type() string { return TypeMsgStopPlan }

// ValidateBasic implements Msg
func (m MsgStopPlan) ValidateBasic() error {
	if m.Owner == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, m.Owner)
	}
	return nil
}

// GetSignBytes implements Msg
func (m MsgCreateSubscription) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&m)
	return sdk.MustSortJSON(bz)
}

// Route implements Msg
func (m MsgCreateSubscription) Route() string { return RouterKey }

// Type implements Msg
func (m MsgCreateSubscription) Type() string { return TypeMsgCreateSubscription }

// ValidateBasic implements Msg
func (m MsgCreateSubscription) ValidateBasic() error {
	if m.Subscriber == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, m.Subscriber)
	}
	return nil
}

// GetSignBytes implements Msg
func (m MsgStopSubscription) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&m)
	return sdk.MustSortJSON(bz)
}

// Route implements Msg
func (m MsgStopSubscription) Route() string { return RouterKey }

// Type implements Msg
func (m MsgStopSubscription) Type() string { return TypeMsgStopSubscription }

// ValidateBasic implements Msg
func (m MsgStopSubscription) ValidateBasic() error {
	if m.Subscriber == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, m.Subscriber)
	}
	return nil
}

// GetSignBytes implements Msg
func (m MsgStopUserSubscription) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&m)
	return sdk.MustSortJSON(bz)
}

// Route implements Msg
func (m MsgStopUserSubscription) Route() string { return RouterKey }

// Type implements Msg
func (m MsgStopUserSubscription) Type() string { return TypeMsgStopSubscription }

// ValidateBasic implements Msg
func (m MsgStopUserSubscription) ValidateBasic() error {
	if m.PlanOwner == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, m.PlanOwner)
	}
	return nil
}

func (m MsgCreatePlan) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Owner)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}

func (m MsgStopPlan) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Owner)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}

func (m MsgCreateSubscription) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Subscriber)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}

func (m MsgStopSubscription) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Subscriber)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}

func (m MsgStopUserSubscription) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.PlanOwner)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}
