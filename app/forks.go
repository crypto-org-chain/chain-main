package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var SlashingForkHeights = map[string]int64{
	"testnet-croeseid-4":                    14857500,
	"tempcrypto-org-chain-mainnet-dryrun-1": 6782000,
	"crypto-org-chain-mainnet-1":            16978800,
}

func SlashingForkEnabled(ctx sdk.Context) bool {
	return ctx.BlockHeight() >= SlashingForkHeights[ctx.ChainID()]
}
