package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

var _ types.QueryServer = queryServer{}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
	k Keeper
}

func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

func (q queryServer) AllTierPositions(ctx context.Context, req *types.QueryAllTierPositionsRequest) (*types.QueryAllTierPositionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

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

func (q queryServer) TierPositionsByOwner(ctx context.Context, req *types.QueryTierPositionsByOwnerRequest) (*types.QueryTierPositionsByOwnerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	owner, err := sdk.AccAddressFromBech32(req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %s", err)
	}

	positions, pageResp, err := query.CollectionPaginate(
		ctx,
		q.k.PositionsByOwner,
		req.Pagination,
		func(key collections.Pair[sdk.AccAddress, uint64], _ collections.NoValue) (types.Position, error) {
			return q.k.getPosition(ctx, key.K2())
		},
		query.WithCollectionPaginationPairPrefix[sdk.AccAddress, uint64](owner),
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierPositionsByOwnerResponse{Positions: positions, Pagination: pageResp}, nil
}

func (q queryServer) TierPosition(ctx context.Context, req *types.QueryTierPositionRequest) (*types.QueryTierPositionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	pos, err := q.k.getPosition(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	return &types.QueryTierPositionResponse{Position: pos}, nil
}

func (q queryServer) Tiers(ctx context.Context, _ *types.QueryTiersRequest) (*types.QueryTiersResponse, error) {
	var tiers []types.Tier
	err := q.k.Tiers.Walk(ctx, nil, func(_ uint32, tier types.Tier) (bool, error) {
		tiers = append(tiers, tier)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return &types.QueryTiersResponse{Tiers: tiers}, nil
}

func (q queryServer) RewardsPoolBalance(ctx context.Context, _ *types.QueryRewardsPoolBalanceRequest) (*types.QueryRewardsPoolBalanceResponse, error) {
	addr := q.k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	balances := q.k.bankKeeper.SpendableCoins(ctx, addr)
	return &types.QueryRewardsPoolBalanceResponse{Address: addr.String(), Balance: balances}, nil
}

// EstimatePositionRewards estimates pending base and bonus rewards for a position.
func (q queryServer) EstimatePositionRewards(ctx context.Context, req *types.QueryEstimatePositionRewardsRequest) (*types.QueryEstimatePositionRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	pos, err := q.k.getPosition(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}

	_, baseRewards, bonusRewards, err := q.k.claimRewardsForPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	return &types.QueryEstimatePositionRewardsResponse{
		BaseRewards:  baseRewards,
		BonusRewards: bonusRewards,
	}, nil
}

func (q queryServer) VotingPowerByOwner(ctx context.Context, req *types.QueryVotingPowerByOwnerRequest) (*types.QueryVotingPowerByOwnerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	owner, err := sdk.AccAddressFromBech32(req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %s", err)
	}

	power, err := q.k.getVotingPowerByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}

	return &types.QueryVotingPowerByOwnerResponse{VotingPower: power}, nil
}

func (q queryServer) TotalDelegatedVotingPower(ctx context.Context, req *types.QueryTotalDelegatedVotingPowerRequest) (*types.QueryTotalDelegatedVotingPowerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	power, err := q.k.totalDelegatedVotingPower(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryTotalDelegatedVotingPowerResponse{VotingPower: power}, nil
}
