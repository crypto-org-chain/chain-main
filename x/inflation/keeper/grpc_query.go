package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = Keeper{}

// Params returns the inflation parameters
func (k Keeper) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(sdkCtx)
	if err != nil {
		return nil, status.Error(codes.NotFound, "failed to get params: "+err.Error())
	}

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}
