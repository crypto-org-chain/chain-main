package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) InterchainAccountAddress(goCtx context.Context, req *types.QueryInterchainAccountAddressRequest) (*types.QueryInterchainAccountAddressResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	icaAddress, err := k.GetInterchainAccountAddress(ctx, req.ConnectionId, req.Owner)
	if err != nil {
		return nil, err
	}

	return &types.QueryInterchainAccountAddressResponse{
		InterchainAccountAddress: icaAddress,
	}, nil
}
