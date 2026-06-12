package testutil

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var TestBlockHeaderHash = []byte{0xfa, 0xce, 0xfe, 0xed}

var TestOwner = sdk.AccAddress([]byte("test_owner__"))

func DelegatorAddress(owner sdk.AccAddress, id uint64) string {
	return types.DerivePositionDelegatorAddress(TestBlockHeaderHash, owner, id, 0).String()
}
