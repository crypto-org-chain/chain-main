package keeper

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
)

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	return types.NewParams(
		k.MinTimeoutDuration(ctx),
	)
}

// SetParams set the params
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramStore.SetParamSet(ctx, &params)
}

// MinTimeoutDuration returns the MinTimeoutDuration param
func (k Keeper) MinTimeoutDuration(ctx sdk.Context) (res time.Duration) {
	k.paramStore.Get(ctx, types.KeyMinTimeoutDuration, &res)
	return
}
