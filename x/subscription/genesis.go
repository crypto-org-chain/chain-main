package subscription

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v2/x/subscription/keeper"
	"github.com/crypto-org-chain/chain-main/v2/x/subscription/types"
)

// InitGenesis initializes the capability module's state from a provided genesis
// state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, data types.GenesisState) {
	k.SetPlanID(ctx, data.StartingPlanId)
	k.SetSubscriptionID(ctx, data.StartingSubscriptionId)
	k.SetParams(ctx, &data.Params)
}

// ExportGenesis returns the capability module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	startingPlanID, err := k.GetPlanID(ctx)
	if err != nil {
		panic(err)
	}
	startingSubscriptionID, err := k.GetSubscriptionID(ctx)
	if err != nil {
		panic(err)
	}
	plans := []types.Plan{}
	k.IteratePlans(ctx, func(plan types.Plan) bool {
		plans = append(plans, plan)
		return false
	})
	subscriptions := []types.Subscription{}
	k.IterateSubscriptionByCollectionTime(ctx, func(subscription types.Subscription) bool {
		subscriptions = append(subscriptions, subscription)
		return false
	})
	return &types.GenesisState{
		StartingPlanId:         startingPlanID,
		StartingSubscriptionId: startingSubscriptionID,
		Plans:                  plans,
		Subscriptions:          subscriptions,
	}
}
