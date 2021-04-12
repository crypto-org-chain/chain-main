<!--
order: 5
-->

# Parameters

The subscription module contains the following parameters:

| Key                 | Type | Default |
| ------------------- | ---- | ------- |
| GasPerCollection    | int  | 31288   |
| SubscriptionEnabled | bool | true    |
| FailureTolerance    | int  | 3      |

- `GasPerCollection`, gas consumed when creating subscription for each payment collection during it's lifetime.
- `SubscriptionEnabled`, when not enabled, disable action executed in begin blocker, disable create plans/subscriptions, but allow stop plans/subscriptions.
- `FailureTolerance`, consecutive failure times allowed for subscription, after consecutive `FailureTolerance` failures happens, the subscription is deleted automatically.

## Estimate `GasPerCollection`

- Size of plan: `a`
  - `5000(max description) + 140(max title) = 5140` (ignore other fields)
- Size of subscription: `b`
  - `int64 * 5 + int32 + bech32 address = 40 + 4 + 42 = 86`
- `RoundUp` cpu cost: `c`
  - `1000`, (arbitrary)
- `read_flat/write_flat/read_per_byte/write_per_byte/delete/iter_next`, from [`KVGasConfig`](https://github.com/cosmos/cosmos-sdk/blob/master/store/types/gas.go#L165)

### Create subscription

- RoundUp cpu cost, `c`

### Begin block

- iteration, `iter_next`.
- load subscription, `read_flat + read_per_byte * b`
- load plan, `read_flat + read_per_byte * a`
- coin transfer (ignore the bytes to keep it simple)
  - load account meta, `read_flat`
  - for each denom:
    - load balance, `read_flat`
    - set balance, `write_flat`
- update next collection time
  - RoundUp cpu cost, `c`
  - delete in queue, `delete`
  - insert in queue, `write_flat`
  - Store subscription, `write_flat + write_per_byte * b`

### Sum up

```
read_flat * 4 +
write_flat * 3 +
read_per_byte * (a + b) +
write_per_byte * b +
iter_next +
delete +
c * 2
= 1000 * 4 +
  2000 * 3 +
  3 * (5140 + 86) +
  30 * 86 +
  30 +
  1000 +
  1000 * 2
= 31288
```

### Other aspects ignored
- the bytes in kv operations of coin transfer.
- failed collection and retries.
