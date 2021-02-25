<!--
order: 5
-->

# Parameters

The subscription module contains the following parameters:

| Key                 | Type | Example |
| ------------------- | ---- | ------- |
| GasPerCollection    | int  | "10000" |
| SubscriptionEnabled | bool | true    |
| FailureTolerance    | int  | 30      |

- `GasPerCollection`, gas consumed when creating subscription for each payment collection during it's lifetime.
- `SubscriptionEnabled`, when not enabled, disable action executed in begin blocker, disable create plans/subscriptions, but allow stop plans/subscriptions.
- `FailureTolerance`, consecutive failure times allowed for subscription, after consecutive `FailureTolerance` failures happens, the subscription is deleted automatically.