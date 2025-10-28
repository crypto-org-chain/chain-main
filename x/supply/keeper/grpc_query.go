package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/supply/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TotalSupply implements the Query/TotalSupply gRPC method
func (k Keeper) TotalSupply(ctx context.Context, _ *types.SupplyRequest) (*types.SupplyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	totalSupply := k.GetTotalSupply(sdkCtx)

	return &types.SupplyResponse{Supply: totalSupply}, nil
}

// LiquidSupply implements the Query/LiquidSupply gRPC method
func (k Keeper) LiquidSupply(ctx context.Context, _ *types.SupplyRequest) (*types.SupplyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	liquidSupply := k.GetLiquidSupply(sdkCtx)

	return &types.SupplyResponse{Supply: liquidSupply}, nil
}
