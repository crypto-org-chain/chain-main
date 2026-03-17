package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"github.com/cosmos/cosmos-sdk/types/query"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = &queryServer{}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{k: k}
}

type queryServer struct {
	types.UnimplementedQueryServer
	k Keeper
}

// Params returns the tieredrewards module parameters.
func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// AllTierPositions returns all positions with pagination.
func (q queryServer) AllTierPositions(ctx context.Context, req *types.QueryAllTierPositionsRequest) (*types.QueryAllTierPositionsResponse, error) {
	positions, pageResp, err := query.CollectionPaginate(
		ctx,
		q.k.Positions,
		req.Pagination,
		func(_ uint64, pos types.Position) (types.Position, error) {
			return pos, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &types.QueryAllTierPositionsResponse{
		Positions:  positions,
		Pagination: pageResp,
	}, nil
}

// TierPositionsByOwner returns all positions for a given owner address.
func (q queryServer) TierPositionsByOwner(ctx context.Context, req *types.QueryTierPositionsByOwnerRequest) (*types.QueryTierPositionsByOwnerResponse, error) {
	owner, err := sdk.AccAddressFromBech32(req.Owner)
	if err != nil {
		return nil, err
	}

	positions, err := q.k.GetPositionsByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierPositionsByOwnerResponse{Positions: positions}, nil
}

// TierPosition returns a single position by ID.
func (q queryServer) TierPosition(ctx context.Context, req *types.QueryTierPositionRequest) (*types.QueryTierPositionResponse, error) {
	pos, err := q.k.Positions.Get(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	return &types.QueryTierPositionResponse{Position: pos}, nil
}
