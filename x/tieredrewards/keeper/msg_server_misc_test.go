package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

var testFunder = sdk.AccAddress([]byte("test_funder_________")).String()

func (s *KeeperSuite) TestFundTierPool_Success() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

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

func (s *KeeperSuite) TestFundTierPool_ZeroAmount() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    sdk.NewCoins(),
	}

	_, err := msgServer.FundTierPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidCoins)
}

func (s *KeeperSuite) TestFundTierPool_InsufficientFunds() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

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

func (s *KeeperSuite) TestFundTierPool_RejectsNonBondDenom() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	nonBondDenom := "utest"

	fundAmount := sdk.NewCoins(sdk.NewCoin(nonBondDenom, sdkmath.NewInt(5000)))
	funderAddr, _ := sdk.AccAddressFromBech32(testFunder)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, funderAddr, fundAmount)
	s.Require().NoError(err)

	msg := &types.MsgFundTierPool{
		Depositor: testFunder,
		Amount:    fundAmount,
	}

	_, err = msgServer.FundTierPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrInvalidAmount)
	s.Require().ErrorContains(err, bondDenom)
}
