package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-com/chain-main/x/supply/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

// NewQuerier returns a new sdk.Keeper instance.
func NewQuerier(k Keeper, legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		switch path[0] {
		case types.QueryTotalSupply:
			return queryTotalSupply(ctx, req, k, legacyQuerierCdc)
		case types.QueryLiquidSupply:
			return queryLiquidSupply(ctx, req, k, legacyQuerierCdc)

		default:
			return nil, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unknown %s query endpoint: %s", types.ModuleName, path[0])
		}
	}
}

func queryTotalSupply(
	ctx sdk.Context,
	req abci.RequestQuery,
	k Keeper,
	legacyQuerierCdc *codec.LegacyAmino,
) ([]byte, error) {
	totalSupply := k.GetTotalSupply(ctx)

	res, err := legacyQuerierCdc.MarshalJSON(totalSupply)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return res, nil
}

func queryLiquidSupply(
	ctx sdk.Context,
	req abci.RequestQuery,
	k Keeper,
	legacyQuerierCdc *codec.LegacyAmino,
) ([]byte, error) {
	liquidSupply := k.GetLiquidSupply(ctx)

	res, err := legacyQuerierCdc.MarshalJSON(liquidSupply)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return res, nil
}
