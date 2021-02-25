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

- `@yearly`: `0 0 1 1 *`
- `@monthly`: `0 0 1 * *`
- `@weekly`: `0 0 * * 1`
- `@daily`: `0 0 * * *`
- `@hourly`: `0 * * * *`

### Invalid intervals

There are some crontab specifications which don't have any matches, they are forbidden, for example:

- `0 0 30 2 *` Invalid, there's no 30days in February.
- `0 0 31 2,4,6,9,11 *` Invalid, all the valid monthes don't have 31 days.
- `0 0 29 2 *` Valid, it happens in leap years only.

When specify month days and week days together, the result is the intersection of both rules:

- `0 0 29 2 1` Valid, it happens when 29.2 also happens to be Monday.