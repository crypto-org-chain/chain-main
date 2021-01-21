<!--
order: 3
-->

# Messages

<<<<<<< HEAD
## MsgCreatePlan

Merchant or payment provider create plans.

```protobuf
message MsgCreatePlan {
  string title;
  string description;
  Decimal price,  // price in fiat currency show to user
  string fiat_currency,  // fiat currency name
  string token_denom,  // denomination of token
  Decimal conversion_rate,  // conversion rate from fiat currency to token, maintained by owner
  int32 interval
}
```

The message signer is the owner.

## MsgStopPlan

```protobuf
message MsgStopPlan {
  int32 planId = 1,
}
```

Can only stopped by the owner of the plan, also removes all subscriptions.

## MsgChangeConversionRate

```protobuf
message MsgChangeConversionRate {
  int32 planId = 1,
  Decimal rate = 2,
}
```

Can only be changed by plan owner.

## MsgCreateSubscription

```protobuf
message MsgCreateSubscription {
  int32 planId = 1,
}
```

The message signer subscribe the plan.

## MsgStopSubscription

```protobuf
message MsgStopSubscription {
  int32 planId = 1,
  Address subscriber = 2,
}
```

The subscriber unsubscribe the plan, or plan owner cancel a subscription.

## MsgCollectPayments

```protobuf
message MsgCollectPayments {
  int32 planId = 1,
}
```

Can only issued by plan owner.

`x/subscription` module automatically collect payments from subscribers at begin block, and write collection results into events. Merchants or payment providers can watch the events, if there's failed payment, they can choose to stop the off-chain service or cancel the subscription.

```python
plan = get_plan(planId)
current_period = (block_time + plan.timezone) / plan.interval
if current_period > plan.last_collection_period:
    amount = plan.price * plan.rate
    for subscription in get_subscriptions(plan.id):
        if transfer(subscription.subscriber, plan.owner, amount, plan.token_denom):
            emit Event(
                'collect_payment',
                planId=plan.id,
                subscriber=subscription.subscriber,
                amount=amount,
            )
    plan.last_collection_period = current_period
```

=======
## MsgCreateSubscriptionPlan

Plan owners create plans, the message signer is the plan owner.

```protobuf
message MsgCreateSubscriptionPlan {
  string title;
  string description;
  Coins price,  // amount + denomination, amount + denomination, ...
  int32 subscription_duration,
  repeated int32 collection_timestamps,
}
```

## MsgStopSubscriptionPlan

Sent by plan owners, also removes all corresponding subscriptions.

```protobuf
message MsgStopSubscriptionPlan {
  int32 plan_id,
}
```

## MsgCreateSubscription

The message signer subscribe to the plan.

```protobuf
message MsgCreateSubscription {
  int32 plan_id,
}
```

It'll consume some gases for each collection period:

```python
ConsumeGas( count_period(create_time, expiration_time, plan.cron_spec, plan.cron_tz) * GasPerCollection )
```

## MsgStopSubscription

Both the subscriber and the plan owner can stop a subscription.

```protobuf
message MsgStopSubscription {
  int32 plan_id,
  Address subscriber,
}
```
>>>>>>> eb2ff2349109aef578f4961c65e1dcf1ad89fdad
