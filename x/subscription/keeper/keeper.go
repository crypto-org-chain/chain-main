package keeper

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/crypto-org-chain/chain-main/v1/x/subscription/types"
)

type (
	// Keeper of the chainmain store
	Keeper struct {
		cdc        codec.BinaryMarshaler
		paramSpace paramtypes.Subspace
		storeKey   sdk.StoreKey
		sendKeeper types.SendKeeper
	}
)

// NewKeeper creates a chainmain keeper
func NewKeeper(
	cdc codec.BinaryMarshaler, storeKey sdk.StoreKey, paramSpace paramtypes.Subspace,
	sendKeeper types.SendKeeper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}
	return Keeper{
		cdc:        cdc,
		paramSpace: paramSpace,
		storeKey:   storeKey,
		sendKeeper: sendKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	k.paramSpace.SetParamSet(ctx, params)
}

func (k Keeper) GetParams(ctx sdk.Context, params *types.Params) {
	k.paramSpace.GetParamSet(ctx, params)
}

// GetPlanID gets the highest plan ID
func (k Keeper) GetPlanID(ctx sdk.Context) (planID uint64, err error) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.PlanIDKey)
	if bz == nil {
		return 0, sdkerrors.Wrap(types.ErrInvalidGenesis, "initial plan ID hasn't been set")
	}

	planID = types.GetPlanIDFromBytes(bz)
	return planID, nil
}

// SetPlanID sets the new plan ID to the store
func (k Keeper) SetPlanID(ctx sdk.Context, planID uint64) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.PlanIDKey, types.GetPlanIDBytes(planID))
}

func (k Keeper) GetPlan(ctx sdk.Context, planID uint64) (types.Plan, bool) {
	store := ctx.KVStore(k.storeKey)

	bz := store.Get(types.PlanKey(planID))
	if bz == nil {
		return types.Plan{}, false
	}

	var plan types.Plan
	k.MustUnmarshalPlan(bz, &plan)
	return plan, true
}

func (k Keeper) SetPlan(ctx sdk.Context, plan types.Plan) {
	store := ctx.KVStore(k.storeKey)
	bz := k.MustMarshalPlan(plan)
	store.Set(types.PlanKey(plan.PlanId), bz)
}

func (k Keeper) CreatePlan(ctx sdk.Context, msg types.MsgCreatePlan) (types.Plan, error) {
	planID, err := k.GetPlanID(ctx)
	if err != nil {
		return types.Plan{}, err
	}
	plan := types.Plan{
		Owner:        msg.Owner,
		Title:        msg.Title,
		Description:  msg.Description,
		Price:        msg.Price,
		PlanId:       planID,
		DurationSecs: msg.DurationSecs,
		CronSpec:     msg.CronSpec,
	}

	if planID+1 < planID {
		return types.Plan{}, errors.New("planID overflow")
	}
	k.SetPlan(ctx, plan)
	k.SetPlanID(ctx, planID+1)

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeCreatePlan,
			sdk.NewAttribute(types.AttributeKeyPlanID, strconv.FormatUint(plan.PlanId, 10)),
		),
	})
	return plan, nil
}

func (k Keeper) StopPlan(ctx sdk.Context, planID uint64) {
	store := ctx.KVStore(k.storeKey)

	// remove all subscriptions
	k.IterateSubscriptionsOfPlan(ctx, planID, func(subscription types.Subscription) bool {
		k.RemoveSubscription(ctx, subscription)
		return false
	})

	store.Delete(types.PlanKey(planID))
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeStopPlan,
			sdk.NewAttribute(types.AttributeKeyPlanID, strconv.FormatUint(planID, 10)),
		),
	})
}

// GetSubscriptionID gets the highest subscription ID
func (k Keeper) GetSubscriptionID(ctx sdk.Context) (subscriptionID uint64, err error) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.SubscriptionIDKey)
	if bz == nil {
		return 0, sdkerrors.Wrap(types.ErrInvalidGenesis, "initial subscription ID hasn't been set")
	}

	subscriptionID = types.GetSubscriptionIDFromBytes(bz)
	return subscriptionID, nil
}

// SetSubscriptionID sets the new subscription ID to the store
func (k Keeper) SetSubscriptionID(ctx sdk.Context, subscriptionID uint64) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.SubscriptionIDKey, types.GetSubscriptionIDBytes(subscriptionID))
}

func (k Keeper) CreateSubscription(ctx sdk.Context, msg types.MsgCreateSubscription) (types.Subscription, error) {
	plan, success := k.GetPlan(ctx, msg.PlanId)
	if !success {
		return types.Subscription{}, errors.New("plan not exists")
	}
	blockTime := ctx.BlockTime().Unix()
	subscriptionID, err := k.GetSubscriptionID(ctx)
	if err != nil {
		return types.Subscription{}, err
	}
	subscription := types.Subscription{
		SubscriptionId:     subscriptionID,
		PlanId:             msg.PlanId,
		Subscriber:         msg.Subscriber,
		CreateTime:         uint64(blockTime),
		NextCollectionTime: uint64(plan.CronSpec.Compile().RoundUp(blockTime+1, plan.Tzoffset)),
		ExpirationTime:     uint64(blockTime) + uint64(plan.DurationSecs),
		PaymentFailures:    0,
	}
	if subscriptionID+1 < subscriptionID {
		return types.Subscription{}, errors.New("subscriptionID overflow")
	}
	k.SetSubscription(ctx, subscription)
	k.SetSubscriptionID(ctx, subscriptionID+1)

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeCreateSubscription,
			sdk.NewAttribute(types.AttributeKeySubscriptionID, strconv.FormatUint(subscription.SubscriptionId, 10)),
		),
	})

	var gasPerCollection uint32
	k.paramSpace.Get(ctx, types.KeyGasPerCollection, &gasPerCollection)
	ctx.GasMeter().ConsumeGas(
		sdk.Gas(gasPerCollection)*plan.CronSpec.Compile().CountPeriods(
			int64(subscription.CreateTime), int64(subscription.ExpirationTime), plan.Tzoffset,
		), "create subscription")
	return subscription, nil
}

func (k Keeper) RemoveSubscription(ctx sdk.Context, subscription types.Subscription) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.SubscriptionKey(subscription.SubscriptionId))
	store.Delete(types.SubscriptionPlanKey(subscription.PlanId, subscription.SubscriptionId))
	store.Delete(types.SubscriptionExpirationKey(subscription.SubscriptionId, subscription.ExpirationTime))
	store.Delete(types.SubscriptionCollectionTimeKey(subscription.SubscriptionId, subscription.NextCollectionTime))
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeStopSubscription,
			sdk.NewAttribute(types.AttributeKeyPlanID, strconv.FormatUint(subscription.PlanId, 10)),
			sdk.NewAttribute(types.AttributeKeySubscriptionID, strconv.FormatUint(subscription.SubscriptionId, 10)),
			sdk.NewAttribute(types.AttributeKeySubscriber, subscription.Subscriber),
		),
	})
}

func (k Keeper) MarshalPlan(plan types.Plan) ([]byte, error) {
	bz, err := k.cdc.MarshalBinaryBare(&plan)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func (k Keeper) UnmarshalPlan(bz []byte, plan *types.Plan) error {
	return k.cdc.UnmarshalBinaryBare(bz, plan)
}

func (k Keeper) MustMarshalPlan(plan types.Plan) []byte {
	bz, err := k.MarshalPlan(plan)
	if err != nil {
		panic(err)
	}
	return bz
}

func (k Keeper) MustUnmarshalPlan(bz []byte, plan *types.Plan) {
	err := k.UnmarshalPlan(bz, plan)
	if err != nil {
		panic(err)
	}
}

func (k Keeper) MustMarshalSubscription(sub types.Subscription) []byte {
	bz, err := k.cdc.MarshalBinaryBare(&sub)
	if err != nil {
		panic(err)
	}
	return bz
}
func (k Keeper) MustUnmarshalSubscription(bz []byte, sub *types.Subscription) {
	err := k.cdc.UnmarshalBinaryBare(bz, sub)
	if err != nil {
		panic(err)
	}
}

func (k Keeper) PlanIterator(ctx sdk.Context) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	return store.Iterator(types.PlanKeyPrefix, sdk.PrefixEndBytes(types.PlanKey(math.MaxUint64)))
}

func (k Keeper) GetSubscription(ctx sdk.Context, subscriptionID uint64) (types.Subscription, bool) {
	store := ctx.KVStore(k.storeKey)

	bz := store.Get(types.SubscriptionKey(subscriptionID))
	if bz == nil {
		return types.Subscription{}, false
	}

	var sub types.Subscription
	k.MustUnmarshalSubscription(bz, &sub)
	return sub, true
}

// SetSubscription sets the new plan ID to the store
func (k Keeper) SetSubscription(ctx sdk.Context, subscription types.Subscription) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.SubscriptionKey(subscription.SubscriptionId), k.MustMarshalSubscription(subscription))
	store.Set(types.SubscriptionPlanKey(subscription.PlanId, subscription.SubscriptionId), []byte{})
	store.Set(types.SubscriptionExpirationKey(subscription.SubscriptionId, subscription.ExpirationTime), []byte{})
	store.Set(types.SubscriptionCollectionTimeKey(subscription.SubscriptionId, subscription.NextCollectionTime), []byte{})
}

func (k Keeper) UpdateCollectionTime(ctx sdk.Context, subscription *types.Subscription, collectionTime uint64) {
	store := ctx.KVStore(k.storeKey)
	if collectionTime != subscription.NextCollectionTime {
		store.Delete(types.SubscriptionCollectionTimeKey(subscription.SubscriptionId, subscription.NextCollectionTime))
		subscription.NextCollectionTime = collectionTime
		store.Set(types.SubscriptionKey(subscription.SubscriptionId), k.MustMarshalSubscription(*subscription))
		store.Set(types.SubscriptionCollectionTimeKey(subscription.SubscriptionId, subscription.NextCollectionTime), []byte{})
	}
}

func (k Keeper) SubscriptonExpirationIterator(ctx sdk.Context) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	return store.Iterator(types.SubscriptionExpirationKeyPrefix, sdk.PrefixEndBytes(types.SubscriptionExpirationKey(math.MaxUint64, math.MaxUint64)))
}

func (k Keeper) IterateSubscriptionByExpirationTime(ctx sdk.Context, cb func(subscription types.Subscription) (stop bool)) {
	iterator := k.SubscriptonExpirationIterator(ctx)

	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		id, _ := types.SplitSubscriptionExpirationKey(iterator.Key())
		subscription, found := k.GetSubscription(ctx, id)
		if !found {
			panic(errors.New("subscription not found, corrupted database"))
		}
		if cb(subscription) {
			break
		}
	}
}

func (k Keeper) SubscriptonCollectionTimeIterator(ctx sdk.Context) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	return store.Iterator(types.SubscriptionCollectionTimeKeyPrefix, sdk.PrefixEndBytes(types.SubscriptionCollectionTimeKey(math.MaxUint64, math.MaxUint64)))
}

func (k Keeper) IterateSubscriptionByCollectionTime(ctx sdk.Context, cb func(subscription types.Subscription) (stop bool)) {
	iterator := k.SubscriptonCollectionTimeIterator(ctx)

	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		subscriptionID, _ := types.SplitSubscriptionCollectionTimeKey(iterator.Key())
		subscription, found := k.GetSubscription(ctx, subscriptionID)
		if !found {
			panic(errors.New("subscription not found, corrupted database"))
		}
		if cb(subscription) {
			break
		}
	}
}

func (k Keeper) SubscriptonPlanIterator(ctx sdk.Context, planID uint64) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	prefix := types.SubscriptionKeyPrefixForPlan(planID)
	return store.Iterator(prefix, sdk.PrefixEndBytes(types.SubscriptionPlanKey(planID, math.MaxUint64)))
}

func (k Keeper) IterateSubscriptionsOfPlan(ctx sdk.Context, planID uint64, cb func(subscription types.Subscription) (stop bool)) {
	iterator := k.SubscriptonPlanIterator(ctx, planID)

	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		subscriptionID, _ := types.SplitSubscriptionPlanKey(iterator.Key())
		subscription, found := k.GetSubscription(ctx, subscriptionID)
		if !found {
			panic(errors.New("subscription not found, corrupted database"))
		}
		if cb(subscription) {
			break
		}
	}
}

func (k Keeper) IteratePlans(ctx sdk.Context, cb func(plan types.Plan) (stop bool)) {
	var plan types.Plan
	iterator := k.PlanIterator(ctx)

	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		k.MustUnmarshalPlan(iterator.Value(), &plan)
		if cb(plan) {
			break
		}
	}
}

func (k Keeper) TryCollect(ctx sdk.Context, subscription types.Subscription) bool {
	blockTime := ctx.BlockTime().Unix()
	if uint64(blockTime) < subscription.NextCollectionTime {
		return false
	}
	plan, exists := k.GetPlan(ctx, subscription.PlanId)
	if !exists {
		return false
	}
	var failureTolerance uint32
	k.paramSpace.Get(ctx, types.KeyFailureTolorance, &failureTolerance)
	subscriber, err := sdk.AccAddressFromBech32(subscription.Subscriber)
	if err != nil {
		return false
	}
	owner, err := sdk.AccAddressFromBech32(plan.Owner)
	if err != nil {
		return false
	}
	err = k.sendKeeper.SendCoins(ctx, subscriber, owner, plan.Price)
	if err != nil {
		// payment fails
		subscription.PaymentFailures++
		if subscription.PaymentFailures > failureTolerance {
			k.RemoveSubscription(ctx, subscription)
		}
	} else {
		k.Logger(ctx).Info("payment success", "subscription", subscription.SubscriptionId, "blockTime", blockTime)
		subscription.PaymentFailures = 0
		ctx.EventManager().EmitEvents(sdk.Events{
			sdk.NewEvent(
				types.EventTypeCollectPayment,
				sdk.NewAttribute(types.AttributeKeyPlanID, strconv.FormatUint(plan.PlanId, 10)),
				sdk.NewAttribute(types.AttributeKeySubscriber, subscription.Subscriber),
				sdk.NewAttribute(types.AttributeKeyAmount, plan.Price.String()),
			),
		})
		k.UpdateCollectionTime(ctx, &subscription, uint64(plan.CronSpec.Compile().RoundUp(blockTime+1, plan.Tzoffset)))
	}
	ctx.KVStore(k.storeKey).Set(types.SubscriptionKey(subscription.SubscriptionId), k.MustMarshalSubscription(subscription))
	return subscription.PaymentFailures == 0
}
