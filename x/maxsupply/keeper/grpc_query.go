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
	params := k.GetParams(sdkCtx)

	return &types.QueryMaxSupplyResponse{
		MaxSupply: params.MaxSupply.String(),
	}, nil
}
