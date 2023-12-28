package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
)

// AccAddress returns a sample account address (for use in testing)
func AccAddress() string {
	pk := ed25519.GenPrivKey().PubKey()
	addr := pk.Address()
	return sdk.AccAddress(addr).String()
}
