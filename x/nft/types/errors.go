// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package types

import (
	sdkerrors "cosmossdk.io/errors"
)

var (
	ErrInvalidCollection = sdkerrors.Register(ModuleNameAlias, 2, "invalid nft collection")
	ErrUnknownCollection = sdkerrors.Register(ModuleNameAlias, 3, "unknown nft collection")
	ErrInvalidNFT        = sdkerrors.Register(ModuleNameAlias, 4, "invalid nft")
	ErrNFTAlreadyExists  = sdkerrors.Register(ModuleNameAlias, 5, "nft already exists")
	ErrUnknownNFT        = sdkerrors.Register(ModuleNameAlias, 6, "unknown nft")
	ErrEmptyTokenData    = sdkerrors.Register(ModuleNameAlias, 7, "nft data can't be empty")
	ErrUnauthorized      = sdkerrors.Register(ModuleNameAlias, 8, "unauthorized address")
	ErrInvalidDenom      = sdkerrors.Register(ModuleNameAlias, 9, "invalid denom")
	ErrInvalidTokenID    = sdkerrors.Register(ModuleNameAlias, 10, "invalid nft id")
	ErrInvalidTokenURI   = sdkerrors.Register(ModuleNameAlias, 11, "invalid nft uri")
	ErrInvalidDenomName  = sdkerrors.Register(ModuleNameAlias, 12, "invalid denom name")
)
