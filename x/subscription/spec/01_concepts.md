<!--
order: 1
-->

# Concepts

## Subscription

- Merchant or payment provider create/stop plans on chain
- User subscribe/unsubscribe to plan
- Merchant or payment provider can collect payments at defined time period and price, the collection results are written into events. The collection can happen once at anytime during current payment period, can't re-collect missed period.
- Merchant or payment provider can unsubscriber user actively, for example when failed to collect payment from them.
- Merchant or payment provider maintains the token-fiat conversion rate, to keep a steady fiat price.

