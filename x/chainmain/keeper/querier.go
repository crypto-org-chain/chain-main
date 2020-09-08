package keeper

import (
	// this line is used by starport scaffolding
	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// NewQuerier creates a new querier for chainmain clients.
func NewQuerier(k Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		switch path[0] { // nolint: gocritic
		// this line is used by starport scaffolding # 2
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "unknown chainmain query endpoint")
		}
	}
}
