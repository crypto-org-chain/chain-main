<!--
order: 2
-->

# State

The `x/subscription` stores the plans and subscriptions on chain:

```golang
type Plan struct {
  id int,  // auto-increasing unique identifier
  title string,
  description string,
  owner Address,  // beneficial owner of the plan
  price Decimal,  // price in fiat currency show to user
  fiat_currency String,  // fiat currency name
  token_denom String,  // denomation of token
  conversion_rate Decimal,  // conversion rate from fiat currency to token, maintained by owner

  interval int, // number of seconds of payment period
  timezone Timezone, // the timezone used to offset the time interval
  last_collection_period int, // period index = (timestamp + timezone_offset) / interval
}

type Subscription struct {
  planId int,
  subscriber Address,
}

type GenesisState struct {
  plans [Plan];
  subscriptions [Subscription];
}
```

