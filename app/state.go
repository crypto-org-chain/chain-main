// Copyright 2016 All in Bits, Inc (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-2023 Crypto.org (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2023-present Cronos Labs (licensed under the Apache License, Version 2.0)
package app

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// AppStateFn returns the initial application state using a genesis or the simulation parameters.
// It panics if the user provides files for both of them.
// If a file is not given for the genesis or the sim params, it creates a randomized one.
// nolint:revive
func AppStateFn(cdc codec.JSONCodec, simManager *module.SimulationManager) simtypes.AppStateFn {
	return simapp.AppStateFnWithExtendedCb(cdc, simManager, NewDefaultGenesisState(cdc), nil)
}
