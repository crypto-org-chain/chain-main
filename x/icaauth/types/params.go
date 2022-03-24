package types

import (
	"fmt"
	"time"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	KeyMinTimeoutDuration     = []byte("MinTimeoutDuration")
	DefaultMinTimeoutDuration = time.Hour
)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams(
	minTimeoutDuration time.Duration,
) Params {
	return Params{
		MinTimeoutDuration: minTimeoutDuration,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(
		DefaultMinTimeoutDuration,
	)
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyMinTimeoutDuration, &p.MinTimeoutDuration, validateMinTimeoutDuration),
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateMinTimeoutDuration(p.MinTimeoutDuration); err != nil {
		return err
	}

	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p) // nolint:errcheck
	return string(out)
}

// validateMinTimeoutDuration validates the MinTimeoutDuration param
func validateMinTimeoutDuration(v interface{}) error {
	_, ok := v.(time.Duration)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	return nil
}
