<!--
order: 2
-->

# State

The `x/subscription` stores the plans and subscriptions on chain:

```golang
type SubscriptionPlan struct {
  id int,  // auto-increasing unique identifier
  title string,
  description string,
  owner Address,  // beneficial owner of the plan
  price Coins,  // price to pay for each period, Coins contains both amount and denomination
  subscription_duration int,  // duration of subscriptions
  cron_spec CronSpec,  // Configure time intervals, parsed from crontab syntax
  cron_tz Timezone,  // timezone for cron_spec
}

type Subscription struct {
  plan_id int,
  subscriber Address,
  create_time int, // the block time when subscription was created
  // the timestamp of last successful collection,
  // default to the current block time round-down against cron spec
  // subscribers don't pay for the period it gets created in
  last_collected_time int,
  payment_failures int,  // times of failed payment collection
}

type GenesisState struct {
  subscription_plans [SubscriptionPlan];
  subscriptions [Subscription];
}

func (s *Subscription) next_collection_time() int {
  var plan = get_plan(s.plan_id)
  return round_up_time(s.last_collected_time, plan.cron_spec, plan.cron_tz)
}

func (s *Subscription) expiration_time() int {
  var plan = get_plan(s.plan_id)
  return s.create_time + plan.subscription_duration
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
def round_down_time(timestamp, cron_spec, cron_tx):
    # return the largest timestamp which matches cron_spec and less or equal than timestamp

def round_up_time(timestamp, cron_spec, cron_tx):
    # return the smallest timestamp which matches cron_spec and greater than timestamp

def count_period(begin_time, end_time, cron_spec, cron_tz):
    # return the number of periods between two timestamps
    count = 0
    while True:
      begin_time = round_up_time(begin_time, cron_spec, cron_tz)
      if begin_time >= end_time:
        break
      count += 1
    return count
```

To keep payment collection idempotent (no duplicated collection in same collection period), we record the last round-down timestamp of collection:

```python
period_index = round_down_time(block_time, cron_spec)
if period_index > last_period_index:
    # do the collection
    last_period_index = period_index
```
