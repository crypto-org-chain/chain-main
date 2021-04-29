// Copyright (c) 2016 Shanghai Bianjie AI Technology Inc.
// Modifications Copyright (c) 2020, Foris Limited (licensed under the Apache License, Version 2.0)
package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewDenom return a new denom
func NewDenom(id, name, schema string, creator sdk.AccAddress) Denom {
	return Denom{
		Id:      id,
		Name:    name,
		Schema:  schema,
		Creator: creator.String(),
	}
}
