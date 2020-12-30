<!--
order: 4
-->

# Events

The `x/subscription` module emits the following events:

## BeginBlocker

| Type              | Attribute Key | Attribute Value |
| ----------------- | ------------- | --------------- |
| collect_payment   | subscriber    | {Address}       |
| collect_payment   | amount        | {Coins}         |
| collect_payment   | plan_id       | {int}           |
| stop_subscription | plan_id       | {int}           |
| stop_subscription | subscriber    | {Address}       |


## MsgStopSubscription

| Type              | Attribute Key | Attribute Value |
| ----------------- | ------------- | --------------- |
| stop_subscription | plan_id       | {int}           |
| stop_subscription | subscriber    | {Address}       |

## MsgCreateSubscription

| Type                | Attribute Key | Attribute Value |
| ------------------- | ------------- | --------------- |
| create_subscription | plan_id       | {int}           |
| create_subscription | subscriber    | {Address}       |

