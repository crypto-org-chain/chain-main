package subscription

import (
	"time"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/chain-main/v1/x/subscription/keeper"
	"github.com/crypto-org-chain/chain-main/v1/x/subscription/types"
)

// BeginBlocker remove expired subscription and collect payments
// on every begin block
func BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock, k keeper.Keeper) {
	var params types.Params
	k.GetParams(ctx, &params)
	if !params.SubscriptionEnabled {
		return
	}
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	k.IterateSubscriptionByExpirationTime(ctx, func(subscription types.Subscription) bool {
		if uint64(ctx.BlockTime().Unix()) >= subscription.ExpirationTime {
			k.RemoveSubscription(ctx, subscription)
			return false
		}
		return true
	})

	k.IterateSubscriptionByCollectionTime(ctx, func(subscription types.Subscription) bool {
		// try to collect payment from subscription
		blockTime := uint64(ctx.BlockTime().Unix())
		if blockTime >= subscription.NextCollectionTime {
			k.TryCollect(ctx, subscription)
			return false
		}
		return true
	})
}
