package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"

	"cosmossdk.io/math"
)

func (s *KeeperSuite) TestUpdateParams_Success() {
	authority := s.keeper.GetAuthority()

	newParams := types.DefaultParams()
	newParams.MaxSupply = math.NewInt(1_000_000_000)

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)

	// Verify params were actually updated in state
	stored, err := s.keeper.GetParams(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(newParams.MaxSupply, stored.MaxSupply)
}

func (s *KeeperSuite) TestUpdateParams_InvalidAuthority() {
	newParams := types.DefaultParams()

	msg := &types.MsgUpdateParams{
		Authority: "cosmos1invalid_authority_addr",
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "invalid authority")
}

func (s *KeeperSuite) TestUpdateParams_NegativeMaxSupply() {
	authority := s.keeper.GetAuthority()

	newParams := types.DefaultParams()
	newParams.MaxSupply = math.NewInt(-1)

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "max supply cannot be negative")
}

func (s *KeeperSuite) TestUpdateParams_InvalidDecayRate() {
	authority := s.keeper.GetAuthority()

	newParams := types.DefaultParams()
	newParams.DecayRate = math.LegacyNewDecWithPrec(150, 2) // 1.5 > 1.0 → invalid

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "decay rate")
}

func (s *KeeperSuite) TestUpdateParams_InvalidDecayStartHeight() {
	authority := s.keeper.GetAuthority()

	newParams := types.DefaultParams()
	newParams.DecayStartHeight = 0 // must be > 0

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "decay start height must be positive")
}

func (s *KeeperSuite) TestUpdateParams_InvalidBurnedAddress() {
	authority := s.keeper.GetAuthority()

	newParams := types.DefaultParams()
	newParams.BurnedAddresses = []string{"not_a_valid_bech32_address"}

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "invalid burned address")
}

func (s *KeeperSuite) TestUpdateParams_DuplicateBurnedAddress() {
	authority := s.keeper.GetAuthority()

	newParams := types.DefaultParams()
	newParams.BurnedAddresses = []string{
		"cosmos139f7kncmglres2nf3h4hc4tade85ekfr8sulz5",
		"cosmos139f7kncmglres2nf3h4hc4tade85ekfr8sulz5",
	}

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "duplicate burned address")
}

func (s *KeeperSuite) TestUpdateParams_ZeroMaxSupply() {
	authority := s.keeper.GetAuthority()

	// Zero max supply means unlimited — should succeed
	newParams := types.DefaultParams()
	newParams.MaxSupply = math.NewInt(0)

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)

	stored, err := s.keeper.GetParams(s.ctx)
	s.Require().NoError(err)
	s.Require().True(stored.MaxSupply.IsZero())
}

func (s *KeeperSuite) TestUpdateParams_DecayRateBoundaries() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// decayRate = 0 (disabled) — should succeed
	params0 := types.DefaultParams()
	params0.DecayRate = math.LegacyZeroDec()
	_, err := msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{Authority: authority, Params: params0})
	s.Require().NoError(err)

	// decayRate = 1.0 (100%) — should succeed
	params1 := types.DefaultParams()
	params1.DecayRate = math.LegacyOneDec()
	_, err = msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{Authority: authority, Params: params1})
	s.Require().NoError(err)
}
