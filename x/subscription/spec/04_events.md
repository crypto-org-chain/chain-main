<!--
order: 4
-->

# Events

The `x/subscription` module emits the following events:

## BeginBlocker

| Type            | Attribute Key | Attribute Value |
| --------------- | ------------- | --------------- |
| collect_payment | subscriber    | {Address}       |
| collect_payment | amount        | {Amount}        |
| collect_payment | planId        | {int}           |

Only succesfully collections are written into events, merchants can query the subscriptions and compare to find out failed subscriptions.