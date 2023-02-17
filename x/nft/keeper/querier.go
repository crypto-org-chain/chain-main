// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package keeper

import (
	"encoding/binary"

	abci "github.com/tendermint/tendermint/abci/types"

	newsdkerrors "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

// NewQuerier is the module level router for state queries
// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func NewQuerier(k Keeper, legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) (res []byte, err error) {
		switch path[0] {
		case types.QuerySupply:
			return querySupply(ctx, req, k, legacyQuerierCdc)
		case types.QueryOwner:
			return queryOwner(ctx, req, k, legacyQuerierCdc)
		case types.QueryCollection:
			return queryCollection(ctx, req, k, legacyQuerierCdc)
		case types.QueryDenom:
			return queryDenom(ctx, req, k, legacyQuerierCdc)
		case types.QueryDenomByName:
			return queryDenomByName(ctx, req, k, legacyQuerierCdc)
		case types.QueryDenoms:
			return queryDenoms(ctx, req, k, legacyQuerierCdc)
		case types.QueryNFT:
			return queryNFT(ctx, req, k, legacyQuerierCdc)
		default:
			return nil, newsdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unknown query path: %s", path[0])
		}
	}
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func querySupply(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	var params types.QuerySupplyParams

	err := legacyQuerierCdc.UnmarshalJSON(req.Data, &params)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	var supply uint64
	if params.Owner.Empty() && len(params.Denom) > 0 {
		supply = k.GetTotalSupply(ctx, params.Denom)
	} else {
		supply = k.GetTotalSupplyOfOwner(ctx, params.Denom, params.Owner)
	}

	bz := make([]byte, 8)
	binary.LittleEndian.PutUint64(bz, supply)
	return bz, nil
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func queryOwner(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	var params types.QueryOwnerParams

	err := legacyQuerierCdc.UnmarshalJSON(req.Data, &params)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	owner, err := k.GetOwner(ctx, params.Owner, params.Denom)

	if err != nil {
		return nil, err
	}

	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, owner)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func queryCollection(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	var params types.QueryCollectionParams

	err := legacyQuerierCdc.UnmarshalJSON(req.Data, &params)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	collection, err := k.GetCollection(ctx, params.Denom)
	if err != nil {
		return nil, err
	}

	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, collection)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func queryDenom(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	var params types.QueryDenomParams

	if err := legacyQuerierCdc.UnmarshalJSON(req.Data, &params); err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	denom, err := k.GetDenom(ctx, params.ID)

	if err != nil {
		return nil, err
	}

	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, denom)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func queryDenomByName(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	var params types.QueryDenomByNameParams

	if err := legacyQuerierCdc.UnmarshalJSON(req.Data, &params); err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	denom, err := k.GetDenomByName(ctx, params.Name)

	if err != nil {
		return nil, err
	}

	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, denom)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func queryDenoms(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	denoms := k.GetDenoms(ctx)

	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, denoms)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func queryNFT(ctx sdk.Context, req abci.RequestQuery, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	var params types.QueryNFTParams

	if err := legacyQuerierCdc.UnmarshalJSON(req.Data, &params); err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	nft, err := k.GetNFT(ctx, params.Denom, params.TokenID)
	if err != nil {
		return nil, newsdkerrors.Wrapf(types.ErrUnknownNFT, "invalid NFT %s from collection %s", params.TokenID, params.Denom)
	}

	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, nft)
	if err != nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}
