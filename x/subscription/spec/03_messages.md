<!--
order: 3
-->

# Messages

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
