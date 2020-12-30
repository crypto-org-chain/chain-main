<!--
order: 1
-->

# Concepts

## Subscription

- Subscription plan owners create/stop plans on Chain, the plan prices are defined for specified token denominations, subscribers should do necessary token conversions in other ways, e.g. exchange.
- Users subscribe/unsubscribe to plan.
- Payments are collected automatically at the start of their specified intervals, the results of collections are written into block events.
- Create plan transaction will consume a gas fee to cover the computational cost of future automatic payment collections.
- If the automatic collection mechanism fails on some subscribers, Chain won't automatically retry later, the corresponding plan owner can either configure their plan to automatically cancel these subscribers or manually retry failed collections later.

### Intervals

Intervals are specified using [crontab syntax](https://crontab.guru/), for example:

- `* * * * *`: At *every minute*
- `0 8 * * *`: At *08**:**00*
- `0 8 1 * *`: At *08**:**00* *on day-of-month 1*
- `0 8 * * 1`: At *08**:**00* *on Monday*

Convenient shortcuts:

- `@monthly`: `0 0 * * *`