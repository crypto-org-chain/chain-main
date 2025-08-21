package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v4/x/maxsupply/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = Keeper{}

// MaxSupply returns the maximum supply
func (k Keeper) MaxSupply(ctx context.Context, req *types.QueryMaxSupplyRequest) (*types.QueryMaxSupplyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(sdkCtx)
	if err != nil {
		return nil, status.Error(codes.NotFound, "failed to get params: "+err.Error())
	}

	return &types.QueryMaxSupplyResponse{
		MaxSupply: params.MaxSupply.String(),
	}, nil
}

// BurnedAddresses returns the list of burned addresses
func (k Keeper) BurnedAddresses(ctx context.Context, req *types.QueryBurnedAddressesRequest) (*types.QueryBurnedAddressesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(sdkCtx)
	if err != nil {
		return nil, status.Error(codes.NotFound, "failed to get params: "+err.Error())
	}

	return &types.QueryBurnedAddressesResponse{
		BurnedAddresses: params.BurnedAddresses,
	}, nil
}
