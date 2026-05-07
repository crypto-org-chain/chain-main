package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestMsgLockTier_Basic() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	resp, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Position should be persisted
	pos, err := s.keeper.GetPositionState(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().Equal(freshAddr.String(), pos.Owner)
	s.Require().True(s.getPositionAmount(pos).Equal(msg.Amount), "derived amount should equal locked amount")
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Delegation.ValidatorAddress)
	s.Require().True(pos.Delegation.Shares.IsPositive())
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))
	s.Require().Equal(uint64(0), pos.LastEventSeq, "LastEventSeq should be 0 for fresh validator")

	valCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount)
}

func (s *KeeperSuite) TestMsgLockTier_LastEventSeqSkipsPriorEvents() {
	// Create a first position to establish validator count.
	lockAmt := sdkmath.NewInt(1000)
	pos1 := s.setupNewTierPosition(lockAmt, false)
	valAddr := sdk.MustValAddressFromBech32(pos1.Delegation.ValidatorAddress)

	// Record a slash event via the staking hook.
	err := s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	// Create a second position via LockTier on the same validator.
	_, bondDenom := s.getStakingData()
	freshAddr := s.fundRandomAddr(bondDenom, lockAmt)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           lockAmt,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	pos2, err := s.keeper.GetPositionState(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// The second position's LastEventSeq should equal 1 (the slash event seq).
	s.Require().Equal(uint64(1), pos2.LastEventSeq,
		"new position should skip prior events; LastEventSeq should be 1 (the slash event)")

	// The first position's LastEventSeq should still be 0 (set at creation, before the slash).
	pos1, err = s.keeper.GetPositionState(s.ctx, pos1.Id)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), pos1.LastEventSeq,
		"first position's LastEventSeq should remain 0")
}

func (s *KeeperSuite) TestMsgLockTier_WithImmediateTriggerExit() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:                  freshAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	}

	resp, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	pos, err := s.keeper.GetPositionState(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
}

func (s *KeeperSuite) TestMsgLockTier_TierNotFound() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               999,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTierNotFound)
}

func (s *KeeperSuite) TestMsgLockTier_TierCloseOnly() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set tier to close only
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	_, err = msgServer.LockTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgLockTier_BelowMinLock() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(999))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(999), // min is 1000
		ValidatorAddress: valAddr.String(),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrMinLockAmountNotMet)
}

func (s *KeeperSuite) TestMsgLockTier_TransfersTokens() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, freshAddr, bondDenom)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, freshAddr, bondDenom)
	s.Require().Equal(sdkmath.NewInt(1000), balBefore.Amount.Sub(balAfter.Amount))
}

// TestMsgLockTier_RoutesBaseRewardsToOwner verifies that a newly created position
// routes the positions delegation rewards to the owner.
func (s *KeeperSuite) TestMsgLockTier_RoutesBaseRewardsToOwner() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	posDelAddr := types.GetDelegatorAddress(resp.PositionId)
	withdrawAddr, err := s.app.DistrKeeper.GetDelegatorWithdrawAddr(s.ctx, posDelAddr)
	s.Require().NoError(err)
	s.Require().Equal(freshAddr.String(), withdrawAddr.String())
}

func (s *KeeperSuite) TestMsgLockTier_ValidatorNotBonded() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.jailAndUnbondValidator(valAddr)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)
}
