package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO decide the fork heights
var SlashingForkHeights = map[string]int64{
	"crypto-org-chain-mainnet-1": 0,
	"testnet-croeseid-4":         0,
}

func SlashingForkEnabled(ctx sdk.Context) bool {
	return SlashingForkHeights[ctx.ChainID()] >= ctx.BlockHeight()
}
