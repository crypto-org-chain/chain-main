package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/crypto-org-chain/chain-main/v1/x/subscription/types"
)

var _ types.QueryServer = Keeper{}

// Plan returns plan details based on PlanID
func (k Keeper) Plan(c context.Context, req *types.QueryPlanRequest) (*types.QueryPlanResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	plan, found := k.GetPlan(ctx, req.PlanId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "plan %d doesn't exist", req.PlanId)
	}

	return &types.QueryPlanResponse{Plan: plan}, nil
}

// Plans implements the Query/Plans gRPC method
func (k Keeper) Plans(c context.Context, req *types.QueryPlansRequest) (*types.QueryPlansResponse, error) {
	var filteredPlans []types.Plan
	ctx := sdk.UnwrapSDKContext(c)

	store := ctx.KVStore(k.storeKey)
	planStore := prefix.NewStore(store, types.PlanKeyPrefix)

	pageRes, err := query.FilteredPaginate(planStore, req.Pagination, func(key []byte, value []byte, accumulate bool) (bool, error) {
		var p types.Plan
		if err := k.cdc.UnmarshalBinaryBare(value, &p); err != nil {
			return false, status.Error(codes.Internal, err.Error())
		}

		matchOwner := true
		if len(req.Owner) > 0 && p.Owner != req.Owner {
			matchOwner = false
		}

		if matchOwner {
			if accumulate {
				filteredPlans = append(filteredPlans, p)
			}

			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryPlansResponse{Plans: filteredPlans, Pagination: pageRes}, nil
}

func (k Keeper) Subscription(c context.Context, req *types.QuerySubscriptionRequest) (*types.QuerySubscriptionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.SubscriptionId == 0 {
		return nil, status.Error(codes.InvalidArgument, "subscription id can not be 0")
	}

	ctx := sdk.UnwrapSDKContext(c)
	subscription, exists := k.GetSubscription(ctx, req.SubscriptionId)
	if !exists {
		return nil, status.Error(codes.InvalidArgument, "subscription don't exists")
	}
	return &types.QuerySubscriptionResponse{Subscription: subscription}, nil
}

// Subscriptions returns subscriptions matching the query
func (k Keeper) Subscriptions(c context.Context, req *types.QuerySubscriptionsRequest) (*types.QuerySubscriptionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	var subscriptions []types.Subscription
	ctx := sdk.UnwrapSDKContext(c)

	store := ctx.KVStore(k.storeKey)
	subscriptionStore := prefix.NewStore(store, types.SubscriptionKeyPrefix)

	pageRes, err := query.FilteredPaginate(subscriptionStore, req.Pagination, func(key []byte, value []byte, accumulate bool) (bool, error) {
		var subscription types.Subscription
		if err := k.cdc.UnmarshalBinaryBare(value, &subscription); err != nil {
			return false, err
		}

		matchPlanID := true
		// negative PlanId means not filtering
		if req.PlanId >= 0 && subscription.PlanId != uint64(req.PlanId) {
			matchPlanID = false
		}

		matchSubscriber := true
		if len(req.Subscriber) > 0 && subscription.Subscriber != req.Subscriber {
			matchSubscriber = false
		}

		if matchPlanID && matchSubscriber {
			if accumulate {
				subscriptions = append(subscriptions, subscription)
			}

			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QuerySubscriptionsResponse{Subscriptions: subscriptions, Pagination: pageRes}, nil
}

// Params queries all params
func (k Keeper) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	var params types.Params
	k.paramSpace.GetParamSet(ctx, &params)
	return &types.QueryParamsResponse{Params: params}, nil
}
