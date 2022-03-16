package keeper

import (
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
)

var _ types.QueryServer = Keeper{}
