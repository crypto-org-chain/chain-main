package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	query "github.com/cosmos/cosmos-sdk/types/query"
)

var _ types.QueryServer = queryServer{}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
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

// TierPoolBalance returns the balance of the tier rewards pool.
func (q queryServer) TierPoolBalance(ctx context.Context, _ *types.QueryTierPoolBalanceRequest) (*types.QueryTierPoolBalanceResponse, error) {
	poolAddr := q.k.accountKeeper.GetModuleAddress(types.TierPoolName)
	balances := q.k.bankKeeper.GetAllBalances(ctx, poolAddr)
	return &types.QueryTierPoolBalanceResponse{Balance: balances}, nil
}

// TierPosition returns a tier position by ID.
func (q queryServer) TierPosition(ctx context.Context, req *types.QueryTierPositionRequest) (*types.QueryTierPositionResponse, error) {
	pos, err := q.k.GetPosition(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	return &types.QueryTierPositionResponse{Position: pos}, nil
}

// TierPositionsByOwner returns all tier positions for an owner, with pagination.
// Uses the owner secondary index for O(K) lookup instead of O(N) full-table scan.
// K is bounded by the number of positions per owner (each requires locked tokens).
func (q queryServer) TierPositionsByOwner(ctx context.Context, req *types.QueryTierPositionsByOwnerRequest) (*types.QueryTierPositionsByOwnerResponse, error) {
	if _, err := sdk.AccAddressFromBech32(req.Owner); err != nil {
		return nil, errors.Wrap(err, "invalid owner address")
	}

	// Use the owner index for efficient lookup.
	allPositions, err := q.k.GetPositionsByOwner(ctx, req.Owner)
	if err != nil {
		return nil, err
	}

	total := uint64(len(allPositions))

	// Support Reverse ordering.
	if req.Pagination != nil && req.Pagination.Reverse {
		for i, j := 0, len(allPositions)-1; i < j; i, j = i+1, j-1 {
			allPositions[i], allPositions[j] = allPositions[j], allPositions[i]
		}
	}

	// Determine start offset (supports both Key-based cursor and Offset pagination).
	var offset, limit uint64
	if req.Pagination != nil {
		if len(req.Pagination.Key) > 0 {
			// Key-based cursor: validate length and find the position matching the cursor.
			if len(req.Pagination.Key) != 8 {
				return nil, errors.Wrap(sdkerrors.ErrInvalidRequest, "pagination key must be 8 bytes (uint64 big-endian)")
			}
			cursor := sdk.BigEndianToUint64(req.Pagination.Key)
			found := false
			for i, pos := range allPositions {
				if pos.PositionId == cursor {
					offset = uint64(i) + 1 // start AFTER the cursor (standard SDK semantics)
					found = true
					break
				}
			}
			if !found {
				// Cursor position no longer exists; return empty result.
				return &types.QueryTierPositionsByOwnerResponse{
					Positions:  []types.TierPosition{},
					Pagination: &query.PageResponse{Total: total},
				}, nil
			}
		} else {
			offset = req.Pagination.Offset
		}
		limit = req.Pagination.Limit
	}
	if limit == 0 {
		limit = query.DefaultLimit
	}

	if offset >= total {
		return &types.QueryTierPositionsByOwnerResponse{
			Positions:  []types.TierPosition{},
			Pagination: &query.PageResponse{Total: total},
		}, nil
	}

	end := offset + limit
	if end < offset || end > total { // overflow guard
		end = total
	}

	// Populate NextKey for cursor-based traversal.
	var nextKey []byte
	if end < total {
		nextKey = sdk.Uint64ToBigEndian(allPositions[end].PositionId)
	}

	return &types.QueryTierPositionsByOwnerResponse{
		Positions: allPositions[offset:end],
		Pagination: &query.PageResponse{
			Total:   total,
			NextKey: nextKey,
		},
	}, nil
}

// AllTierPositions returns all tier positions, with pagination.
func (q queryServer) AllTierPositions(ctx context.Context, req *types.QueryAllTierPositionsRequest) (*types.QueryAllTierPositionsResponse, error) {
	results, pageResp, err := query.CollectionPaginate(ctx, q.k.Positions, req.Pagination,
		func(key uint64, value types.TierPosition) (types.TierPosition, error) {
			return value, nil
		})
	if err != nil {
		return nil, err
	}
	return &types.QueryAllTierPositionsResponse{Positions: results, Pagination: pageResp}, nil
}

// EstimateTierBonus returns estimated pending rewards for a position.
func (q queryServer) EstimateTierBonus(ctx context.Context, req *types.QueryEstimateTierBonusRequest) (*types.QueryEstimateTierBonusResponse, error) {
	pos, err := q.k.GetPosition(ctx, req.PositionId)
	if err != nil {
		return nil, err
	}
	params, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	tier, err := params.GetTierDefinition(pos.TierId)
	if err != nil {
		return nil, err
	}
	bonus, err := q.k.CalculateBonus(ctx, pos, tier)
	if err != nil {
		return nil, err
	}
	bondDenom, err := q.k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, bonus))
	// Base estimate would require actually calling distribution which mutates state,
	// so just return empty for base estimate in query
	return &types.QueryEstimateTierBonusResponse{
		EstimatedBase:  sdk.Coins{},
		EstimatedBonus: bonusCoins,
	}, nil
}

// TierVotingPower returns the tier voting power for an address.
func (q queryServer) TierVotingPower(ctx context.Context, req *types.QueryTierVotingPowerRequest) (*types.QueryTierVotingPowerResponse, error) {
	power, err := q.k.GetVotingPowerForAddress(ctx, req.Owner)
	if err != nil {
		return nil, err
	}
	return &types.QueryTierVotingPowerResponse{VotingPower: power}, nil
}
