package types

import (
	"encoding/binary"
	"fmt"
)

const (
	// ModuleName defines the module name
	ModuleName = "subscription"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName
)

// Keys for subscription store
var (
	// tag -> next_plan_id
	PlanIDKey = []byte{0x00}
	// tag -> next_subscription_id
	SubscriptionIDKey = []byte{0x01}
	// tag + plan_id -> plan
	PlanKeyPrefix = []byte{0x02}
	// tag + subscription_id -> subscription
	SubscriptionKeyPrefix = []byte{0x03}
	// tag + plan_id + subscription_id -> ()
	SubscriptionPlanKeyPrefix = []byte{0x04}
	// tag + expiration_time + subscription_id -> ()
	SubscriptionExpirationKeyPrefix = []byte{0x05}
	// tag + next_collection_time + subscription_id -> ()
	SubscriptionCollectionTimeKeyPrefix = []byte{0x06}
)

// GetPlanIDBytes returns the byte representation of the planID
func GetPlanIDBytes(planID uint64) (planIDBz []byte) {
	planIDBz = make([]byte, 8)
	binary.BigEndian.PutUint64(planIDBz, planID)
	return planIDBz
}

// GetSubscriptionIDBytes returns the byte representation of the subscriptionID
func GetSubscriptionIDBytes(subscriptionID uint64) (subscriptionIDBz []byte) {
	subscriptionIDBz = make([]byte, 8)
	binary.BigEndian.PutUint64(subscriptionIDBz, subscriptionID)
	return subscriptionIDBz
}

// GetPlanIDFromBytes returns planID in uint64 format from a byte array
func GetPlanIDFromBytes(bz []byte) uint64 {
	return binary.BigEndian.Uint64(bz)
}

// GetSubscriptionIDFromBytes returns subscriptionID in uint64 format from a byte array
func GetSubscriptionIDFromBytes(bz []byte) uint64 {
	return binary.BigEndian.Uint64(bz)
}

// PlanKey returns the key of plan
func PlanKey(planID uint64) []byte {
	return append(PlanKeyPrefix, GetPlanIDBytes(planID)...)
}

func SubscriptionPlanKey(planID uint64, subscriptionID uint64) []byte {
	return append(SubscriptionKeyPrefixForPlan(planID), GetSubscriptionIDBytes(subscriptionID)...)
}

func SubscriptionKey(subscriptionID uint64) []byte {
	return append(SubscriptionKeyPrefix, GetSubscriptionIDBytes(subscriptionID)...)
}

// SubscriptionKeyPrefixForPlan returns the key prefix for subscriptions of the plan id
func SubscriptionKeyPrefixForPlan(planID uint64) []byte {
	return append(SubscriptionPlanKeyPrefix, GetPlanIDBytes(planID)...)
}

func SubscriptionExpirationKey(subscriptionID uint64, expirationTime uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, expirationTime)
	return append(append(SubscriptionExpirationKeyPrefix, bz...), GetSubscriptionIDBytes(subscriptionID)...)
}

func SubscriptionCollectionTimeKey(subscriptionID uint64, nextCollectionTime uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, nextCollectionTime)
	return append(append(SubscriptionCollectionTimeKeyPrefix, bz...), GetSubscriptionIDBytes(subscriptionID)...)
}

func SplitSubscriptionExpirationKey(key []byte) (subscriptionID uint64, expirationTime uint64) {
	return splitIDWithTime(key, SubscriptionExpirationKeyPrefix)
}

func SplitSubscriptionCollectionTimeKey(key []byte) (subscriptionID uint64, nextCollectionTime uint64) {
	return splitIDWithTime(key, SubscriptionCollectionTimeKeyPrefix)
}

func SplitSubscriptionPlanKey(key []byte) (subscriptionID uint64, planID uint64) {
	return splitIDWithPlanID(key, SubscriptionPlanKeyPrefix)
}

func splitIDWithTime(key []byte, prefix []byte) (id uint64, time uint64) {
	if len(key) != len(prefix)+8+8 {
		panic(fmt.Sprintf("unexpected key length (%d ≠ %d)", len(key), len(prefix)+8+8))
	}

	time = binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
	id = GetSubscriptionIDFromBytes(key[len(prefix)+8 : len(prefix)+16])
	return
}

func splitIDWithPlanID(key []byte, prefix []byte) (id uint64, planID uint64) {
	if len(key) != len(prefix)+8+8 {
		panic(fmt.Sprintf("unexpected key length (%d ≠ %d)", len(key), len(prefix)+8+8))
	}

	planID = GetPlanIDFromBytes(key[len(prefix) : len(prefix)+8])
	id = GetSubscriptionIDFromBytes(key[len(prefix)+8 : len(prefix)+16])
	return
}
