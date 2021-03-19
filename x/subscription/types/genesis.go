package types

import (
	"errors"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		StartingPlanId:         1,
		StartingSubscriptionId: 1,
		Params:                 DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	for _, plan := range gs.Plans {
		if !plan.CronSpec.Compile().IsValid() {
			return errors.New("invalid plan cronspec")
		}
		if gs.StartingPlanId <= plan.PlanId {
			return errors.New("staring_plan_id not bigger than existing plan id")
		}
	}
	for _, subscription := range gs.Subscriptions {
		if gs.StartingSubscriptionId <= subscription.SubscriptionId {
			return errors.New("staring_subscription_id not bigger than existing subscription id")
		}
	}
	plansMap := map[uint64]*Plan{}
	for i := range gs.Plans {
		plansMap[gs.Plans[i].PlanId] = &gs.Plans[i]
	}
	if len(plansMap) != len(gs.Plans) {
		return errors.New("duplicate plan id")
	}
	for _, subscription := range gs.Subscriptions {
		if _, exists := plansMap[subscription.PlanId]; !exists {
			return errors.New("subscription has invalid plan id")
		}
	}

	subscriptionsMap := map[uint64]*Subscription{}
	for i := range gs.Subscriptions {
		subscriptionsMap[gs.Subscriptions[i].SubscriptionId] = &gs.Subscriptions[i]
	}
	if len(subscriptionsMap) != len(gs.Subscriptions) {
		return errors.New("duplicate subscription id")
	}
	return nil
}
