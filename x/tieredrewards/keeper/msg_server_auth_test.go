package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// --- UpdateParams ---

func (s *KeeperSuite) TestUpdateParams_Success() {
	authority := s.keeper.GetAuthority()
	newParams := types.NewParams(sdkmath.LegacyNewDecWithPrec(5, 2), nil) // 0.05

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)

	stored, err := s.keeper.Params.Get(s.ctx)
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
		Params:    types.NewParams(sdkmath.LegacyNewDec(-1), nil),
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
		Params:    types.NewParams(sdkmath.LegacyZeroDec(), nil),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)
}

// --- AddTier ---

func (s *KeeperSuite) TestAddTier_Success() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgAddTier{
		Authority: authority,
		Tier:      newTestTier(1),
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().NoError(err)

	// Verify tier was stored
	got, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint32(1), got.Id)
	s.Require().True(msg.Tier.BonusApy.Equal(got.BonusApy))
}

func (s *KeeperSuite) TestAddTier_InvalidAuthority() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgAddTier{
		Authority: "cosmos1invalid",
		Tier:      newTestTier(1),
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "invalid authority")
}

func (s *KeeperSuite) TestAddTier_AlreadyExists() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msg := &types.MsgAddTier{
		Authority: authority,
		Tier:      tier,
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierAlreadyExists)
}

func (s *KeeperSuite) TestAddTier_InvalidTier() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgAddTier{
		Authority: authority,
		Tier: types.Tier{
			Id:            1,
			ExitDuration:  0, // invalid
			BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
			MinLockAmount: sdkmath.NewInt(1000),
		},
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "exit duration")
}

// --- UpdateTier ---

func (s *KeeperSuite) TestUpdateTier_Success() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create tier first
	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	// Update bonus APY
	updated := newTestTier(1)
	updated.BonusApy = sdkmath.LegacyNewDecWithPrec(8, 2)

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier:      updated,
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().NoError(err)

	got, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(sdkmath.LegacyNewDecWithPrec(8, 2).Equal(got.BonusApy))
}

func (s *KeeperSuite) TestUpdateTier_SetCloseOnly() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	updated := newTestTier(1)
	updated.CloseOnly = true

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier:      updated,
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().NoError(err)

	got, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(got.CloseOnly)
}

func (s *KeeperSuite) TestUpdateTier_InvalidAuthority() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgUpdateTier{
		Authority: "cosmos1invalid",
		Tier:      newTestTier(1),
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "invalid authority")
}

func (s *KeeperSuite) TestUpdateTier_NotFound() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier:      newTestTier(999),
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestUpdateTier_InvalidTier() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier: types.Tier{
			Id:            1,
			ExitDuration:  time.Hour * 24 * 365,
			BonusApy:      sdkmath.LegacyNewDec(-1), // invalid
			MinLockAmount: sdkmath.NewInt(1000),
		},
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "bonus apy")
}

// --- DeleteTier ---

func (s *KeeperSuite) TestDeleteTier_Success() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        1,
	}

	_, err := msgServer.DeleteTier(s.ctx, msg)
	s.Require().NoError(err)

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}

func (s *KeeperSuite) TestDeleteTier_InvalidAuthority() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgDeleteTier{
		Authority: "cosmos1invalid",
		Id:        1,
	}

	_, err := msgServer.DeleteTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "invalid authority")
}

func (s *KeeperSuite) TestDeleteTier_NotFound() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        999,
	}

	_, err := msgServer.DeleteTier(s.ctx, msg)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestDeleteTier_FailsWithActivePositions() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	// Create a position in tier 1
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        1,
	}

	_, err = msgServer.DeleteTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierHasActivePositions)

	// Tier should still exist
	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestDeleteTier_SucceedsAfterPositionsRemoved() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	// Remove the position
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos))

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        1,
	}

	_, err = msgServer.DeleteTier(s.ctx, msg)
	s.Require().NoError(err)

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}

// --- FundTierPool ---

var testFunder = sdk.AccAddress([]byte("test_funder_________")).String()

func (s *KeeperSuite) whitelistFunder(addr string) {
	params, err := s.keeper.Params.Get(s.ctx)
	s.Require().NoError(err)
	params.PoolFunders = append(params.PoolFunders, addr)
	s.Require().NoError(s.keeper.SetParams(s.ctx, params))
}

func (s *KeeperSuite) TestFundTierPool_Success() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	s.whitelistFunder(testFunder)

	fundAmount := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(5000)))
	funderAddr, _ := sdk.AccAddressFromBech32(testFunder)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, funderAddr, fundAmount)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, bondDenom)

	msg := &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    fundAmount,
	}

	_, err = msgServer.FundTierPool(s.ctx, msg)
	s.Require().NoError(err)

	poolBalAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, bondDenom)
	s.Require().True(poolBalAfter.Amount.Equal(poolBalBefore.Amount.Add(sdkmath.NewInt(5000))),
		"pool balance should have increased by funded amount")
}

func (s *KeeperSuite) TestFundTierPool_NotWhitelisted() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	outsider := sdk.AccAddress([]byte("not_whitelisted_____")).String()

	msg := &types.MsgFundTierPool{
		Depositor: outsider,
		Amount:    sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))),
	}

	_, err = msgServer.FundTierPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrUnauthorizedFunder)
}

func (s *KeeperSuite) TestFundTierPool_ZeroAmount() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.whitelistFunder(testFunder)

	msg := &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(),
	}

	_, err := msgServer.FundTierPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrInvalidAmount)
}

func (s *KeeperSuite) TestFundTierPool_InsufficientFunds() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	s.whitelistFunder(testFunder)

	msg := &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))),
	}

	_, err = msgServer.FundTierPool(s.ctx, msg)
	s.Require().Error(err, "should fail when depositor has insufficient funds")
}

func (s *KeeperSuite) TestFundTierPool_MultipleFunds() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	s.whitelistFunder(testFunder)

	totalFund := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(10000)))
	funderAddr, _ := sdk.AccAddressFromBech32(testFunder)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, funderAddr, totalFund)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, bondDenom)

	_, err = msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(3000))),
	})
	s.Require().NoError(err)

	_, err = msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(7000))),
	})
	s.Require().NoError(err)

	poolBalAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, bondDenom)
	s.Require().True(poolBalAfter.Amount.Equal(poolBalBefore.Amount.Add(sdkmath.NewInt(10000))),
		"pool balance should reflect both deposits")
}

func (s *KeeperSuite) TestFundTierPool_RemovedFromWhitelist() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	s.whitelistFunder(testFunder)

	funderAddr, _ := sdk.AccAddressFromBech32(testFunder)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, funderAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(5000))))
	s.Require().NoError(err)

	_, err = msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))),
	})
	s.Require().NoError(err)

	// Remove funder from whitelist via params update
	params, err := s.keeper.Params.Get(s.ctx)
	s.Require().NoError(err)
	params.PoolFunders = nil
	s.Require().NoError(s.keeper.SetParams(s.ctx, params))

	_, err = msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrUnauthorizedFunder)
}
