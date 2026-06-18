// Cronos.com Chain Copyright 2018-present Cronos.com
package exported

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NFT non fungible token interface
type NFT interface {
	GetID() string
	GetName() string
	GetOwner() sdk.AccAddress
	GetURI() string
	GetData() string
}
