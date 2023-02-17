package types

import (
	sdkerrors "cosmossdk.io/errors"
)

var (
	ErrInvalidClassID      = sdkerrors.Register(ModuleName, 1501, "invalid class id")
	ErrInvalidTokenID      = sdkerrors.Register(ModuleName, 1502, "invalid token id")
	ErrInvalidPacket       = sdkerrors.Register(ModuleName, 1503, "invalid packet")
	ErrTraceNotFound       = sdkerrors.Register(ModuleName, 1504, "class trace not found")
	ErrInvalidVersion      = sdkerrors.Register(ModuleName, 1505, "invalid ICS721 version")
	ErrMaxTransferChannels = sdkerrors.Register(ModuleName, 1506, "max nft-transfer channels")
)
