package types

import "cosmossdk.io/errors"

var (
	ErrBadTransferDelegationSrc     = errors.Register(ModuleName, 1, "transfer delegation source validator not found")
	ErrBadTransferDelegationDest    = errors.Register(ModuleName, 2, "transfer delegation destination validator not found")
	ErrTinyTransferDelegationAmount = errors.Register(ModuleName, 3, "too few tokens to transfer (truncates to zero tokens)")
	ErrTransferDelegationToPoolSelf = errors.Register(ModuleName, 4, "cannot transfer delegation from the pool to itself")
	ErrTierAlreadyExists            = errors.Register(ModuleName, 5, "tier already exists")
	ErrTierHasActivePositions       = errors.Register(ModuleName, 6, "tier has active positions")
	ErrTierIsCloseOnly              = errors.Register(ModuleName, 7, "tier is close only")
	ErrInvalidAmount                = errors.Register(ModuleName, 8, "invalid amount")
	ErrMinLockAmountNotMet          = errors.Register(ModuleName, 9, "min lock amount not met")
	ErrActiveRedelegation           = errors.Register(ModuleName, 10, "cannot transfer delegation with active incoming redelegation")
	ErrValidatorNotBonded           = errors.Register(ModuleName, 11, "validator is not bonded")
)
