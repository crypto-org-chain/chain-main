package keeper

import (
	"context"
	stderrors "errors"

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
		func(_ uint64, pos types.Position) (types.PositionResponse, error) {
			tokenValue, err := q.k.positionTokenValue(ctx, pos)
			if err != nil {
				return types.PositionResponse{}, err
			}
			return pos.ToPositionResponse(tokenValue), nil
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
		func(key collections.Pair[sdk.AccAddress, uint64], _ collections.NoValue) (types.PositionResponse, error) {
			pos, err := q.k.getPosition(ctx, key.K2())
			if err != nil {
				return types.PositionResponse{}, err
			}
			tokenValue, err := q.k.positionTokenValue(ctx, pos)
			if err != nil {
				return types.PositionResponse{}, err
			}
			return pos.ToPositionResponse(tokenValue), nil
		},
		query.WithCollectionPaginationPairPrefix[sdk.AccAddress, uint64](owner),
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierPositionsByOwnerResponse{Positions: positions, Pagination: pageResp}, nil
}

func (q queryServer) TierPositionsByTier(ctx context.Context, req *types.QueryTierPositionsByTierRequest) (*types.QueryTierPositionsByTierResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	positions, pageResp, err := query.CollectionPaginate(
		ctx,
		q.k.PositionsByTier,
		req.Pagination,
		func(key collections.Pair[uint32, uint64], _ collections.NoValue) (types.PositionResponse, error) {
			pos, err := q.k.getPosition(ctx, key.K2())
			if err != nil {
				return types.PositionResponse{}, err
			}
			tokenValue, err := q.k.positionTokenValue(ctx, pos)
			if err != nil {
				return types.PositionResponse{}, err
			}
			return pos.ToPositionResponse(tokenValue), nil
		},
		query.WithCollectionPaginationPairPrefix[uint32, uint64](req.TierId),
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierPositionsByTierResponse{Positions: positions, Pagination: pageResp}, nil
}

func (q queryServer) TierPosition(ctx context.Context, req *types.QueryTierPositionRequest) (*types.QueryTierPositionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	pos, err := q.k.getPosition(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}

	tokenValue, err := q.k.positionTokenValue(ctx, pos)
	if err != nil {
		return nil, err
	}

	return &types.QueryTierPositionResponse{Position: pos.ToPositionResponse(tokenValue)}, nil
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

func (q queryServer) RewardsPoolBalances(ctx context.Context, _ *types.QueryRewardsPoolBalancesRequest) (*types.QueryRewardsPoolBalancesResponse, error) {
	addr := q.k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	balances := q.k.bankKeeper.GetAllBalances(ctx, addr)
	return &types.QueryRewardsPoolBalancesResponse{Address: addr.String(), Balances: balances}, nil
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

	_, baseRewards, bonusRewards, err := q.k.claimRewards(ctx, pos)
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

func (q queryServer) RawTierPosition(ctx context.Context, req *types.QueryRawTierPositionRequest) (*types.QueryRawTierPositionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	pos, err := q.k.getPosition(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	return &types.QueryRawTierPositionResponse{Position: pos}, nil
}

func (q queryServer) RawTierPositionsByOwner(ctx context.Context, req *types.QueryRawTierPositionsByOwnerRequest) (*types.QueryRawTierPositionsByOwnerResponse, error) {
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
	return &types.QueryRawTierPositionsByOwnerResponse{
		Positions:  positions,
		Pagination: pageResp,
	}, nil
}

func (q queryServer) RawTierPositionsByTier(ctx context.Context, req *types.QueryRawTierPositionsByTierRequest) (*types.QueryRawTierPositionsByTierResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	positions, pageResp, err := query.CollectionPaginate(
		ctx,
		q.k.PositionsByTier,
		req.Pagination,
		func(key collections.Pair[uint32, uint64], _ collections.NoValue) (types.Position, error) {
			return q.k.getPosition(ctx, key.K2())
		},
		query.WithCollectionPaginationPairPrefix[uint32, uint64](req.TierId),
	)
	if err != nil {
		return nil, err
	}
	return &types.QueryRawTierPositionsByTierResponse{
		Positions:  positions,
		Pagination: pageResp,
	}, nil
}

func (q queryServer) RawAllTierPositions(ctx context.Context, req *types.QueryRawAllTierPositionsRequest) (*types.QueryRawAllTierPositionsResponse, error) {
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
	return &types.QueryRawAllTierPositionsResponse{
		Positions:  positions,
		Pagination: pageResp,
	}, nil
}

func (q queryServer) ValidatorData(ctx context.Context, req *types.QueryValidatorDataRequest) (*types.QueryValidatorDataResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	valAddr, err := sdk.ValAddressFromBech32(req.Validator)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid validator address: %s", err)
	}

	resp := &types.QueryValidatorDataResponse{}

	count, err := q.k.PositionCountByValidator.Get(ctx, valAddr)
	if err != nil && !stderrors.Is(err, collections.ErrNotFound) {
		return nil, err
	}
	resp.PositionCount = count

	rng := collections.NewPrefixedPairRange[sdk.ValAddress, uint64](valAddr)
	err = q.k.ValidatorEvents.Walk(ctx, rng, func(_ collections.Pair[sdk.ValAddress, uint64], event types.ValidatorEvent) (bool, error) {
		resp.Events = append(resp.Events, event)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	seq, err := q.k.ValidatorEventSeq.Get(ctx, valAddr)
	if err != nil && !stderrors.Is(err, collections.ErrNotFound) {
		return nil, err
	}
	resp.EventCurrentSeq = seq

	return resp, nil
}

func (q queryServer) PositionMappings(ctx context.Context, req *types.QueryPositionMappingsRequest) (*types.QueryPositionMappingsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	resp := &types.QueryPositionMappingsResponse{}

	unbondIter, err := q.k.UnbondingDelegationMappings.Indexes.ByPosition.MatchExact(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	defer unbondIter.Close()
	for ; unbondIter.Valid(); unbondIter.Next() {
		pk, err := unbondIter.PrimaryKey()
		if err != nil {
			return nil, err
		}
		resp.UnbondingIds = append(resp.UnbondingIds, pk)
	}

	redelIter, err := q.k.RedelegationMappings.Indexes.ByPosition.MatchExact(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	defer redelIter.Close()
	for ; redelIter.Valid(); redelIter.Next() {
		pk, err := redelIter.PrimaryKey()
		if err != nil {
			return nil, err
		}
		resp.RedelegationIds = append(resp.RedelegationIds, pk)
	}

	return resp, nil
}
