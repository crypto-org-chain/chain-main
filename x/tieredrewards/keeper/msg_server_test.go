package keeper_test

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// --- MsgLockTier tests ---

func (s *KeeperSuite) TestMsgLockTier_Basic() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	}

	resp, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Position should be persisted
	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().Equal(delAddr.String(), pos.Owner)
	s.Require().True(sdkmath.NewInt(1000).Equal(pos.Amount))
	s.Require().False(pos.IsDelegated())
	s.Require().False(pos.IsExiting())
}

func (s *KeeperSuite) TestMsgLockTier_WithValidator() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
}

func (s *KeeperSuite) TestMsgLockTier_WithImmediateTriggerExit() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsExiting())
	s.Require().True(pos.ExitUnlockAt.After(pos.ExitTriggeredAt))
}

func (s *KeeperSuite) TestMsgLockTier_TierNotFound() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     999,
		Amount: sdkmath.NewInt(1000),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestMsgLockTier_TierCloseOnly() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set tier to close only
	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msg := &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgLockTier_BelowMinLock() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(999), // min is 1000
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrMinLockAmountNotMet)
}

func (s *KeeperSuite) TestMsgLockTier_TransfersTokens() {
	delAddr, _, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	msg := &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	}

	_, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(sdkmath.NewInt(1000), balBefore.Amount.Sub(balAfter.Amount))
}

// --- MsgCommitDelegationToTier tests ---

func (s *KeeperSuite) TestMsgCommitDelegationToTier_Basic_PartialCommit() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Get delegation amount before
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	delTokensBefore := val.TokensFromShares(del.Shares).TruncateInt()
	halfShares := del.Shares.Quo(sdkmath.LegacyNewDec(2))
	commitAmount := delTokensBefore.Quo(sdkmath.NewInt(2))

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	}

	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().NoError(err)

	// Position should exist and be delegated
	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().Equal(delAddr.String(), pos.Owner)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.Equal(halfShares))

	// Module should have delegation on the same validator
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	moduleDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, moduleAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(moduleDel.Shares.Equal(halfShares))
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_FullCommit() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	delTokensBefore := val.TokensFromShares(del.Shares).TruncateInt()

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           delTokensBefore,
	}

	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().NoError(err)

	// Position should be delegated
	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())

	// User's delegation should be fully removed
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().Error(err, "user delegation should be removed after full commit")

	// Module should have the full delegation
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	moduleDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, moduleAddr, valAddr)
	s.Require().NoError(err)

	// Re-fetch validator after commit for current exchange rate
	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	moduleDelTokens := valAfter.TokensFromShares(moduleDel.Shares).TruncateInt()
	s.Require().True(moduleDelTokens.Equal(delTokensBefore), "module should have the full delegation")

	// Validator tokens should be unchanged
	s.Require().True(val.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged")
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_WithImmediateTriggerExit() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	commitAmount := val.TokensFromShares(del.Shares).TruncateInt().Quo(sdkmath.NewInt(2))

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress:       delAddr.String(),
		ValidatorAddress:       valAddr.String(),
		Id:                     1,
		Amount:                 commitAmount,
		TriggerExitImmediately: true,
	}

	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().True(pos.IsExiting())
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_TierNotFound() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               999,
		Amount:           sdkmath.NewInt(1000),
	}

	_, err := msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_TierCloseOnly() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
	}

	_, err := msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_BelowMinLock() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(999),
	}

	_, err := msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrMinLockAmountNotMet)
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_NoDelegation() {
	_, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	randomAddr := sdk.AccAddress([]byte("addr_with_no_deleg__"))

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: randomAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
	}

	_, err := msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().Error(err)
}
