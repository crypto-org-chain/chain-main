// Cronos.com Chain Copyright 2018-present Cronos.com
package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewDenom return a new denom
func NewDenom(id, name, schema, uri string, creator sdk.AccAddress) Denom {
	return Denom{
		Id:      id,
		Name:    name,
		Schema:  schema,
		Creator: creator.String(),
		Uri:     uri,
	}
}
