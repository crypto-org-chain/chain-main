package keeper

import (
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v2/x/subscription/types"
)

type msgServer struct {
	Keeper Keeper
}

// NewMsgServerImpl returns an implementation of the subscription MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (k msgServer) CreatePlan(goCtx context.Context, msg *types.MsgCreatePlan) (*types.MsgCreatePlanResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var params types.Params
	k.Keeper.GetParams(ctx, &params)
	if !params.SubscriptionEnabled {
		return nil, types.ErrModuleDisabled
	}

	plan, err := k.Keeper.CreatePlan(ctx, *msg)
	if err != nil {
		return nil, err
	}
	defer telemetry.IncrCounter(1, types.ModuleName, "plan")
	return &types.MsgCreatePlanResponse{
		PlanId: plan.PlanId,
	}, nil
}

func (k msgServer) CreateSubscription(goCtx context.Context, msg *types.MsgCreateSubscription) (*types.MsgCreateSubscriptionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var params types.Params
	k.Keeper.GetParams(ctx, &params)
	if !params.SubscriptionEnabled {
		return nil, types.ErrModuleDisabled
	}

	sub, err := k.Keeper.CreateSubscription(ctx, *msg)
	if err != nil {
		return nil, err
	}
	return &types.MsgCreateSubscriptionResponse{
		SubscriptionId: sub.SubscriptionId,
	}, nil
}

func (k msgServer) StopPlan(goCtx context.Context, msg *types.MsgStopPlan) (*types.MsgStopPlanResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	plan, exists := k.Keeper.GetPlan(ctx, msg.PlanId)
	if !exists {
		return nil, errors.New("subscription not exists")
	}
	if plan.Owner != msg.Owner {
		return nil, errors.New("plan owner don't match")
	}
	k.Keeper.StopPlan(ctx, msg.PlanId)
	return &types.MsgStopPlanResponse{}, nil
}

func (k msgServer) StopSubscription(goCtx context.Context, msg *types.MsgStopSubscription) (*types.MsgStopSubscriptionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	subscription, exists := k.Keeper.GetSubscription(ctx, msg.SubscriptionId)
	if !exists {
		return nil, errors.New("subscription not exists")
	}
	if msg.Subscriber != subscription.Subscriber {
		return nil, errors.New("subscriber not match")
	}
	k.Keeper.RemoveSubscription(ctx, subscription)
	return &types.MsgStopSubscriptionResponse{}, nil
}

func (k msgServer) StopUserSubscription(goCtx context.Context, msg *types.MsgStopUserSubscription) (*types.MsgStopUserSubscriptionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	subscription, exists := k.Keeper.GetSubscription(ctx, msg.SubscriptionId)
	if !exists {
		return nil, errors.New("subscription not exists")
	}
	plan, exists := k.Keeper.GetPlan(ctx, subscription.PlanId)
	if !exists {
		return nil, errors.New("subscription not exists")
	}
	if plan.Owner != msg.PlanOwner {
		return nil, errors.New("plan owner don't match")
	}
	k.Keeper.RemoveSubscription(ctx, subscription)
	return &types.MsgStopUserSubscriptionResponse{}, nil
}
