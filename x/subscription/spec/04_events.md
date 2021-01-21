<!--
order: 4
-->

# Events

The `x/subscription` module emits the following events:

## BeginBlocker

<<<<<<< HEAD
| Type            | Attribute Key | Attribute Value |
| --------------- | ------------- | --------------- |
| collect_payment | subscriber    | {Address}       |
| collect_payment | amount        | {Amount}        |
| collect_payment | planId        | {int}           |

Only succesfully collections are written into events, merchants can query the subscriptions and compare to find out failed subscriptions.
=======
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

>>>>>>> eb2ff2349109aef578f4961c65e1dcf1ad89fdad
