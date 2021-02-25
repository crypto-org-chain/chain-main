<!--
order: 3
-->

# Messages

## MsgCreateSubscriptionPlan

Plan owners create plans, the message signer is the plan owner.

```protobuf
message MsgCreateSubscriptionPlan {
  string owner,  // signer
  string title,
  string description,
  Coins price,  // amount + denomination, amount + denomination, ...
  int32 duration_secs,  // duration of subscription
  CronSpec cron_spec,
}
```

## MsgStopSubscriptionPlan

Sent by plan owners, also removes all corresponding subscriptions.

```protobuf
message MsgStopSubscriptionPlan {
  string owner,  // signer
  int64 plan_id,
}
```

## MsgCreateSubscription

The message signer subscribe to the plan.

```protobuf
message MsgCreateSubscription {
  string subscriber, // signer
  int64 plan_id,
}
```

It'll consume some gases for each collection period:

```python
ConsumeGas( count_period(create_time, expiration_time, plan.cron_spec, plan.cron_tz) * GasPerCollection )
```

## MsgStopSubscription

Subscriber stop a subscription.

```protobuf
message MsgStopSubscription {
  string subscriber, // signer
  int64 subscription_id,
}
```

## MsgStopUserSubscription

The plan owner stop user's subscription.

```protobuf
message MsgStopUserSubscription {
  string plan_owner, // signer
  int64 subscription_id,
}
```

