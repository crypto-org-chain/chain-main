package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"
)

func (s *KeeperSuite) TestUpdateParams_Success() {
	authority := s.keeper.GetAuthority()
	newParams := types.NewParams(sdkmath.LegacyNewDecWithPrec(5, 2)) // 0.05

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)

	stored, err := s.keeper.GetParams(s.ctx)
	s.Require().NoError(err)
	s.Require().True(newParams.TargetBaseRewardsRate.Equal(stored.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestUpdateParams_InvalidAuthority() {
	msg := &types.MsgUpdateParams{
		Authority: "cosmos1invalid",
		Params:    types.DefaultParams(),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "invalid authority")
}

func (s *KeeperSuite) TestUpdateParams_NegativeRate() {
	authority := s.keeper.GetAuthority()
	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    types.NewParams(sdkmath.LegacyNewDec(-1)),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "target base rewards rate cannot be negative")
}

func (s *KeeperSuite) TestUpdateParams_ZeroRate() {
	authority := s.keeper.GetAuthority()
	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    types.NewParams(sdkmath.LegacyZeroDec()),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)
}
