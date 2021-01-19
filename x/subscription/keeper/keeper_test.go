package keeper_test

import (
	"testing"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v1/app"
	"github.com/crypto-org-chain/chain-main/v1/x/subscription"
	"github.com/crypto-org-chain/chain-main/v1/x/subscription/types"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestMarshal(t *testing.T) {
	app := app.Setup(false)
	spec, err := types.ParseCronSpec("0 0 1 1 *")
	require.NoError(t, err)
	price, err := sdk.ParseCoinsNormalized("10basecro")
	require.NoError(t, err)
	plan := types.Plan{
		Owner:        "",
		Title:        "",
		Description:  "",
		Price:        price,
		PlanId:       1,
		DurationSecs: 3600,
		CronSpec:     spec,
	}
	var plan2 types.Plan
	app.SubscriptionKeeper.MustUnmarshalPlan(app.SubscriptionKeeper.MustMarshalPlan(plan), &plan2)
	require.Equal(t, plan, plan2)
}

func TestPlanLifeCycle(t *testing.T) {
	app := app.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	price, err := sdk.ParseCoinsNormalized("1basecro")
	require.NoError(t, err)
	spec, err := types.ParseCronSpec("* * * * *")
	require.NoError(t, err)
	msg := types.MsgCreatePlan{
		Owner:        "",
		Title:        "",
		Description:  "",
		Price:        price,
		DurationSecs: 24 * 3600,
		CronSpec:     spec,
	}
	plan, err := app.SubscriptionKeeper.CreatePlan(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, uint64(1), plan.PlanId)

	plan, err = app.SubscriptionKeeper.CreatePlan(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, uint64(2), plan.PlanId)

	plan2, exists := app.SubscriptionKeeper.GetPlan(ctx, 1)
	require.Equal(t, true, exists)
	require.Equal(t, uint64(1), plan2.PlanId)
	plan2, exists = app.SubscriptionKeeper.GetPlan(ctx, 2)
	require.Equal(t, true, exists)
	require.Equal(t, uint64(2), plan2.PlanId)

	plans := []*types.Plan{}
	app.SubscriptionKeeper.IteratePlans(ctx, func(plan types.Plan) bool {
		plans = append(plans, &plan)
		return false
	})
	require.Equal(t, 2, len(plans))

	app.SubscriptionKeeper.StopPlan(ctx, 1)
	app.SubscriptionKeeper.StopPlan(ctx, 2)

	_, exists = app.SubscriptionKeeper.GetPlan(ctx, 1)
	require.Equal(t, false, exists)
	_, exists = app.SubscriptionKeeper.GetPlan(ctx, 2)
	require.Equal(t, false, exists)
}

func TestSubscriptionLifeCycle(t *testing.T) {
	app := app.Setup(false)
	blockTime := time.Date(2020, 1, 0, 0, 0, 1, 0, time.UTC)
	duration := uint32(365 * 24 * 3600)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{
		Time: blockTime,
	})

	price, err := sdk.ParseCoinsNormalized("1basecro")
	require.NoError(t, err)
	spec, err := types.ParseCronSpec("* * * * *")
	require.NoError(t, err)
	msg := types.MsgCreatePlan{
		Owner:        "",
		Title:        "",
		Description:  "",
		Price:        price,
		DurationSecs: duration,
		CronSpec:     spec,
	}
	plan, err := app.SubscriptionKeeper.CreatePlan(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, uint64(1), plan.PlanId)

	subscription, err := app.SubscriptionKeeper.CreateSubscription(ctx, types.MsgCreateSubscription{
		Subscriber: "",
		PlanId:     plan.PlanId,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), subscription.SubscriptionId)
	require.Equal(t, uint64(time.Date(2020, 1, 0, 0, 1, 0, 0, time.UTC).Unix()), subscription.NextCollectionTime)
	require.Equal(t, uint64(blockTime.Unix())+uint64(duration), subscription.ExpirationTime)

	subscription2, err := app.SubscriptionKeeper.CreateSubscription(ctx, types.MsgCreateSubscription{
		Subscriber: "",
		PlanId:     plan.PlanId,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), subscription2.SubscriptionId)

	app.SubscriptionKeeper.StopPlan(ctx, plan.PlanId)

	_, exists := app.SubscriptionKeeper.GetPlan(ctx, plan.PlanId)
	require.Equal(t, false, exists)
	_, exists = app.SubscriptionKeeper.GetSubscription(ctx, subscription.SubscriptionId)
	require.Equal(t, false, exists)
	_, exists = app.SubscriptionKeeper.GetSubscription(ctx, subscription2.SubscriptionId)
	require.Equal(t, false, exists)
}

func TestSubscriptionExpire(t *testing.T) {
	app := app.Setup(false)
	blockTime := time.Date(2020, 1, 0, 0, 0, 1, 0, time.UTC)
	duration := uint32(365 * 24 * 3600)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{
		Time: blockTime,
	})

	price, err := sdk.ParseCoinsNormalized("1basecro")
	require.NoError(t, err)
	spec, err := types.ParseCronSpec("* * * * *")
	require.NoError(t, err)
	msg := types.MsgCreatePlan{
		Owner:        "",
		Title:        "",
		Description:  "",
		Price:        price,
		DurationSecs: duration,
		CronSpec:     spec,
	}
	plan, err := app.SubscriptionKeeper.CreatePlan(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, uint64(1), plan.PlanId)

	sub, err := app.SubscriptionKeeper.CreateSubscription(ctx, types.MsgCreateSubscription{
		Subscriber: "",
		PlanId:     plan.PlanId,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), sub.SubscriptionId)
	require.Equal(t, uint64(time.Date(2020, 1, 0, 0, 1, 0, 0, time.UTC).Unix()), sub.NextCollectionTime)
	require.Equal(t, uint64(blockTime.Unix())+uint64(duration), sub.ExpirationTime)

	sub2, err := app.SubscriptionKeeper.CreateSubscription(ctx, types.MsgCreateSubscription{
		Subscriber: "",
		PlanId:     plan.PlanId,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), sub2.SubscriptionId)

	blockTime = blockTime.Add(time.Second * time.Duration(duration))
	ctx = app.BaseApp.NewContext(false, tmproto.Header{
		Time: blockTime,
	})
	subscription.BeginBlocker(ctx, abci.RequestBeginBlock{
		Header: tmproto.Header{
			Time: blockTime,
		},
	}, app.SubscriptionKeeper)

	// expired
	_, exists := app.SubscriptionKeeper.GetSubscription(ctx, sub.SubscriptionId)
	require.Equal(t, false, exists)
	_, exists = app.SubscriptionKeeper.GetSubscription(ctx, sub2.SubscriptionId)
	require.Equal(t, false, exists)
}

func TestSubscriptionCollect(t *testing.T) {
	a := app.Setup(false)
	time1 := time.Date(2020, 1, 0, 0, 0, 1, 0, time.UTC)
	duration := uint32(365 * 24 * 3600)
	ctx := a.BaseApp.NewContext(false, tmproto.Header{
		Time: time1,
	})
	price, err := sdk.ParseCoinsNormalized("1basecro")
	require.NoError(t, err)
	amount, err := sdk.ParseCoinsNormalized("1000basecro")
	require.NoError(t, err)
	owner := app.AddTestAddrs(a, ctx, 1, sdk.NewCoins())[0]
	subscriber1 := app.AddTestAddrs(a, ctx, 1, amount)[0]
	subscriber2 := app.AddTestAddrs(a, ctx, 1, sdk.NewCoins())[0]

	spec, err := types.ParseCronSpec("0 0 29 2 *")
	require.NoError(t, err)
	msg := types.MsgCreatePlan{
		Owner:        owner.String(),
		Title:        "",
		Description:  "",
		Price:        price,
		DurationSecs: duration,
		CronSpec:     spec,
	}
	plan, err := a.SubscriptionKeeper.CreatePlan(ctx, msg)
	require.NoError(t, err)
	sub1, err := a.SubscriptionKeeper.CreateSubscription(ctx, types.MsgCreateSubscription{
		Subscriber: subscriber1.String(),
		PlanId:     plan.PlanId,
	})
	require.NoError(t, err)
	sub2, err := a.SubscriptionKeeper.CreateSubscription(ctx, types.MsgCreateSubscription{
		Subscriber: subscriber2.String(),
		PlanId:     plan.PlanId,
	})
	require.NoError(t, err)

	time2 := time.Date(2020, 2, 29, 0, 0, 1, 0, time.UTC)
	ctx = a.BaseApp.NewContext(false, tmproto.Header{
		Time: time2,
	})

	subscription.BeginBlocker(ctx, abci.RequestBeginBlock{}, a.SubscriptionKeeper)

	sub1, _ = a.SubscriptionKeeper.GetSubscription(ctx, sub1.SubscriptionId)
	sub2, _ = a.SubscriptionKeeper.GetSubscription(ctx, sub2.SubscriptionId)
	require.Equal(t, uint32(0), sub1.PaymentFailures)
	require.Equal(t, uint32(1), sub2.PaymentFailures)
}
