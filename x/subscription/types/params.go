package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Parameter store keys
var (
	KeyGasPerCollection    = []byte("GasPerCollection")
	KeySubscriptionEnabled = []byte("SubscriptionEnabled")
	KeyFailureTolerance    = []byte("FailureTolerance")
)

// Default parameter values
const (
	DefaultGasPerCollection    uint32 = 1000
	DefaultSubscriptionEnabled bool   = true
	DefaultFailureTolerance    uint32 = 30
)

// Implements params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyGasPerCollection, &p.GasPerCollection, validateIsInt),
		paramtypes.NewParamSetPair(KeySubscriptionEnabled, &p.SubscriptionEnabled, validateIsBool),
		paramtypes.NewParamSetPair(KeyFailureTolerance, &p.FailureTolerance, validateIsInt),
	}
}

func validateIsBool(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateIsInt(i interface{}) error {
	_, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

// ParamKeyTable for auth module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{
		GasPerCollection:    DefaultGasPerCollection,
		SubscriptionEnabled: DefaultSubscriptionEnabled,
		FailureTolerance:    DefaultFailureTolerance,
	}
}
