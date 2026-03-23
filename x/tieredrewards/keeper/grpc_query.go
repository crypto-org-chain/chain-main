package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var _ types.QueryServer = queryServer{}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
	k Keeper
}

// Params returns the tieredrewards module parameters.
func (q queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	params, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// AllTierPositions returns all positions with pagination.
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

// TierPositionsByOwner returns all positions for a given owner address.
func (q queryServer) TierPositionsByOwner(ctx context.Context, req *types.QueryTierPositionsByOwnerRequest) (*types.QueryTierPositionsByOwnerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

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
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	pos, err := q.k.Positions.Get(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	return &types.QueryTierPositionResponse{Position: pos}, nil
}

// Tiers returns all tier definitions.
func (q queryServer) Tiers(ctx context.Context, req *types.QueryTiersRequest) (*types.QueryTiersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

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

// TierPoolBalance returns the current balance of the bonus rewards pool.
func (q queryServer) TierPoolBalance(ctx context.Context, req *types.QueryTierPoolBalanceRequest) (*types.QueryTierPoolBalanceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	poolAddr := q.k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	balances := q.k.bankKeeper.SpendableCoins(ctx, poolAddr)
	return &types.QueryTierPoolBalanceResponse{Balance: balances}, nil
}

// EstimateTierRewards estimates pending base and bonus rewards for a position.
// Base rewards are computed from the stored cumulative ratio (excludes
// rewards accrued since the last UpdateBaseRewardsPerShare).
// Bonus rewards are computed from the position's last accrual time to now.
func (q queryServer) EstimateTierRewards(ctx context.Context, req *types.QueryEstimateTierRewardsRequest) (*types.QueryEstimateTierRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	pos, err := q.k.Positions.Get(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}

	baseRewards := sdk.NewCoins()
	bonusRewards := sdk.NewCoins()

	if !pos.IsDelegated() {
		return &types.QueryEstimateTierRewardsResponse{
			BaseRewards:  baseRewards,
			BonusRewards: bonusRewards,
		}, nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return nil, err
	}

	// Estimate base rewards using the stored ratio
	currentRatio, err := q.k.GetValidatorRewardRatio(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	delta := currentRatio.Sub(pos.BaseRewardsPerShare)
	if delta.IsAllPositive() {
		baseRewards, _ = delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()
	}

	// Estimate bonus rewards
	tier, err := q.k.Tiers.Get(ctx, pos.TierId)
	if err != nil {
		return nil, err
	}

	val, err := q.k.stakingKeeper.GetValidator(ctx, valAddr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		val = stakingtypes.Validator{Tokens: math.ZeroInt()}
	} else if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	bonus := q.k.calculateBonus(pos, val, tier, sdkCtx.BlockTime())
	if bonus.IsPositive() {
		bondDenom, err := q.k.stakingKeeper.BondDenom(ctx)
		if err != nil {
			return nil, err
		}
		bonusRewards = sdk.NewCoins(sdk.NewCoin(bondDenom, bonus))
	}

	return &types.QueryEstimateTierRewardsResponse{
		BaseRewards:  baseRewards,
		BonusRewards: bonusRewards,
	}, nil
}
