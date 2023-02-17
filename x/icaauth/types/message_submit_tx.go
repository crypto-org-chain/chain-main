package types

import (
	"time"

	newsdkerrors "cosmossdk.io/errors"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgSubmitTx = "submit_tx"

var (
	_ sdk.Msg                          = &MsgSubmitTx{}
	_ cdctypes.UnpackInterfacesMessage = &MsgSubmitTx{}
)

func NewMsgSubmitTx(owner string, connectionID string, msgs []sdk.Msg, timeoutDuration *time.Duration) *MsgSubmitTx {
	msgsAny := make([]*cdctypes.Any, len(msgs))
	for i, msg := range msgs {
		any, err := cdctypes.NewAnyWithValue(msg)
		if err != nil {
			panic(err)
		}

		msgsAny[i] = any
	}

	return &MsgSubmitTx{
		Owner:           owner,
		ConnectionId:    connectionID,
		Msgs:            msgsAny,
		TimeoutDuration: timeoutDuration,
	}
}

func (msg MsgSubmitTx) GetMessages() ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, len(msg.Msgs))
	for i, msgAny := range msg.Msgs {
		msg, ok := msgAny.GetCachedValue().(sdk.Msg)
		if !ok {
			return nil, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "messages contains %T which is not a sdk.Msg", msgAny)
		}
		msgs[i] = msg
	}

	return msgs, nil
}

func (msg MsgSubmitTx) CalculateTimeoutDuration(minTimeoutDuration time.Duration) time.Duration {
	var timeoutDuration time.Duration

	if msg.TimeoutDuration != nil && *msg.TimeoutDuration >= minTimeoutDuration {
		timeoutDuration = *msg.TimeoutDuration
	} else {
		timeoutDuration = minTimeoutDuration
	}

	return timeoutDuration
}

func (msg *MsgSubmitTx) Route() string {
	return RouterKey
}

func (msg *MsgSubmitTx) Type() string {
	return TypeMsgSubmitTx
}

func (msg *MsgSubmitTx) GetSigners() []sdk.AccAddress {
	owner, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{owner}
}

func (msg *MsgSubmitTx) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSubmitTx) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return newsdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address (%s)", err)
	}
	return nil
}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (msg MsgSubmitTx) UnpackInterfaces(unpacker cdctypes.AnyUnpacker) error {
	for _, x := range msg.Msgs {
		var msg sdk.Msg
		err := unpacker.UnpackAny(x, &msg)
		if err != nil {
			return err
		}
	}

	return nil
}
