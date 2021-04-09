package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/subscription module sentinel errors
var (
	ErrUnknownPlan                 = sdkerrors.Register(ModuleName, 2, "unknown plan")
	ErrInvalidGenesis              = sdkerrors.Register(ModuleName, 3, "invalid genesis state")
	ErrInvalidPlanContent          = sdkerrors.Register(ModuleName, 4, "invalid plan content")
	ErrInvalidSubscriptionDuration = sdkerrors.Register(ModuleName, 5, "invalid subscription duration")
	ErrInvalidCronSpec             = sdkerrors.Register(ModuleName, 6, "invalid cron spec")
	ErrModuleDisabled              = sdkerrors.Register(ModuleName, 7, "module disabled")
)
