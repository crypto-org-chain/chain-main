package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	owner, err := sdk.AccAddressFromBech32(req.Owner)
	if err != nil {
		return nil, err
	}

	positions, err := q.k.getPositionsByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierPositionsByOwnerResponse{Positions: positions}, nil
}

func (q queryServer) TierPosition(ctx context.Context, req *types.QueryTierPositionRequest) (*types.QueryTierPositionResponse, error) {
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

func (q queryServer) TierVotingPower(ctx context.Context, req *types.QueryTierVotingPowerRequest) (*types.QueryTierVotingPowerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	voter, err := sdk.AccAddressFromBech32(req.Voter)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid voter address: %s", err)
	}

	power, err := q.k.getVotingPowerForAddress(ctx, voter)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierVotingPowerResponse{VotingPower: power}, nil
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
