<!--
order: 2
-->

# State

The `x/subscription` stores the plans and subscriptions on chain:

```golang
type SubscriptionPlan struct {
  plan_id int,  // auto-increasing unique identifier
  title string,
  description string,
  owner Address,  // beneficial owner of the plan
  price Coins,  // price to pay for each period, Coins contains both amount and denomination
  duration_secs int,  // duration of subscriptions
  cron_spec CronSpec,  // Configure time intervals, parsed from crontab syntax
  cron_tz Timezone,  // timezone for cron_spec
}

type Subscription struct {
  subscription_id int,
  plan_id int,
  subscriber Address,
  create_time int, // the block time when subscription was created
  expiration_time int,  // create_time + plan.duration_secs
  // the timestamp of next collection,
  // default to "round_up_time(block_time+1, plan.cron_spec, plan.cron_tz)""
  // so subscriber doesn't pay for the current period it gets created in
  next_collection_time int,
  payment_failures int,  // times of failed payment collection
}

type GenesisState struct {
  subscription_plans [SubscriptionPlan];
  subscriptions [Subscription];
}

// Parsed crontab syntax
type CronSpec struct {
  minute [CronValue]
  hour [CronValue]
  day [CronValue]
  month [CronValue]
  wday [CronValue]
}

CronValue = Any | Range(start, end, step) | Value(v)
```

## Round time

We define two functions to round timestamp to the boundary of periods:

```python
def round_up_time(timestamp, cron_spec, cron_tz):
    # return the smallest timestamp which matches cron_spec and greater than or equal to timestamp

def count_period(begin_time, end_time, cron_spec, cron_tz):
    # return the number of periods between two timestamps
    count = 0
    while True:
      begin_time = round_up_time(begin_time+1, cron_spec, cron_tz)
      if begin_time >= end_time:
        break
      count += 1
    return count
```

We compute and store the next collection time of subscriptions, and only execute collection when block time is greater than that:

```python
next_collection_time = round_up_time(block_time+1, cron_spec, cron_tz)
```
