// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewDenom return a new denom
func NewDenom(id, name, schema string, uri string, creator sdk.AccAddress) Denom {
	return Denom{
		Id:      id,
		Name:    name,
		Schema:  schema,
		Creator: creator.String(),
		Uri:     uri,
	}
}
