<!--
order: 6
-->

# BeginBlock

## Remove expired subscriptions

```python
for plan_id, subscriber, expiration_time in get_subscriptions_sorted_by_expiration_time():
    if block_time >= expiration_time:
        stop_subscription(plan_id, subscriber)
    else:
        break
```

## Collect payments

`x/subscription` module automatically collect payments from subscribers at begin block, and write collection results into events.

```python
for subscription in get_subscriptions_sorted_by_next_collection_time():
    plan = get_plan(subscription.plan_id)
    if block_time >= subscription.next_collection_time:
        if transfer(subscription.subscriber, plan.owner, plan.price):
            subscription.payment_failures = 0
            emit Event(
                    'collect_payment',
                plan_id=plan.id,
                subscriber=subscription.subscriber,
                amount=amount,
            )
            subscription.next_collection_time = round_up_time(block_time, plan.cron_spec, plan.cron_tz)
        else:
            subscription.payment_failures += 1
            if subscription.payment_failures > FailureTolerance:
                stop_subscription(plan.id, subscription.subscriber)
                emit Event(
                    'stop_subscription',
                    plan_id=plan.id,
                    subscriber=subscription.subscriber,
                )
    else:
        break
```
