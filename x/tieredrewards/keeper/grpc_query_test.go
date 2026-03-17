package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"
)

// --- Params ---

func (s *KeeperSuite) TestGRPCQueryParams() {
	customParams := types.NewParams(
		sdkmath.LegacyNewDecWithPrec(3, 2),
		nil,
	)
	s.keeper.InitGenesis(s.ctx, &types.GenesisState{Params: customParams})

	resp, err := s.queryClient.Params(s.ctx.Context(), &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().True(customParams.TargetBaseRewardsRate.Equal(resp.Params.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestGRPCQueryParams_Default() {
	defaultGenesis := types.DefaultGenesisState()
	s.keeper.InitGenesis(s.ctx, defaultGenesis)

	resp, err := s.queryClient.Params(s.ctx.Context(), &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.Params.TargetBaseRewardsRate.IsZero())
}

// --- TierPosition ---

func (s *KeeperSuite) TestGRPCQueryTierPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	resp, err := s.queryClient.TierPosition(s.ctx.Context(), &types.QueryTierPositionRequest{PositionId: 1})
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), resp.Position.Id)
	s.Require().Equal(testPositionOwner, resp.Position.Owner)
	s.Require().True(pos.Amount.Equal(resp.Position.Amount))
}

func (s *KeeperSuite) TestGRPCQueryTierPosition_NotFound() {
	_, err := s.queryClient.TierPosition(s.ctx.Context(), &types.QueryTierPositionRequest{PositionId: 999})
	s.Require().Error(err)
}
