package types

const (
	AttributeKeyPlanID         = "plan_id"
	AttributeKeySubscriptionID = "subscription_id"
	AttributeKeySubscriber     = "subscriber"
	AttributeKeyAmount         = "amount"
)

const (
	EventTypeCreatePlan         = "create_plan"
	EventTypeStopPlan           = "stop_plan"
	EventTypeCreateSubscription = "create_subscription"
	EventTypeStopSubscription   = "stop_subscription"
	EventTypeCollectPayment     = "collect_payment"
)
