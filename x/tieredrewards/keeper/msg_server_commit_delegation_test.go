package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestMsgCommitDelegationToTier_Basic_PartialCommit() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
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
	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().Equal(delAddr.String(), pos.Owner)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.Equal(halfShares))
	s.Require().Equal(uint64(0), pos.LastEventSeq, "LastEventSeq should be 0 for fresh validator")

	// The position's delegator address holds the delegation.
	posDelAddr := types.GetDelegatorAddress(pos.Id)
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(posDel.Shares.Equal(halfShares))

	// Verify that the module account does not hold a delegation.
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, moduleAddr, valAddr)
	s.Require().Error(err, "module account should not hold delegation")

	// Verify that the distribution rewards for this delegation are routed to the owner.
	withdrawAddr, err := s.app.DistrKeeper.GetDelegatorWithdrawAddr(s.ctx, posDelAddr)
	s.Require().NoError(err)
	s.Require().Equal(delAddr.String(), withdrawAddr.String(), "withdraw addr should route to owner")
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_FullCommit() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
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
	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())

	// User's delegation should be fully removed
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().Error(err, "user delegation should be removed after full commit")

	// The position's delegator address holds the delegation.
	posDelAddr := types.GetDelegatorAddress(pos.Id)
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)

	// Re-fetch validator after commit for current exchange rate
	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	posDelTokens := valAfter.TokensFromShares(posDel.Shares).TruncateInt()
	s.Require().True(posDelTokens.Equal(delTokensBefore), "position's delegator address should have the full delegation")

	// Verify that the module account does not hold a delegation.
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, moduleAddr, valAddr)
	s.Require().Error(err, "module account should not hold delegation")

	// Validator tokens should be unchanged
	s.Require().True(val.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged")
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_WithImmediateTriggerExit() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
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

	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_TierNotFound() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
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
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
	}

	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_BelowMinLock() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
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
	s.setupTier(1)
	_, valAddr := s.getDelegator()
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

// TestMsgCommitDelegationToTier_LastEventSeqSkipsPriorEvents verifies that a
// position created via CommitDelegationToTier gets LastEventSeq set to the
// latest event, skipping any prior events on the validator.
func (s *KeeperSuite) TestMsgCommitDelegationToTier_LastEventSeqSkipsPriorEvents() {
	delAddr, _ := s.getDelegator()
	// Create a first position to establish validator count.
	pos1 := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos1.Validator)

	// Record a slash event via the staking hook.
	err := s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	commitAmount := val.TokensFromShares(del.Shares).TruncateInt().Quo(sdkmath.NewInt(2))

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.CommitDelegationToTier(s.ctx, &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	})
	s.Require().NoError(err)

	pos2, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// The new position's LastEventSeq should equal 1 (the slash event).
	s.Require().Equal(uint64(1), pos2.LastEventSeq,
		"new position should skip prior events; LastEventSeq should be 1")

	// The first position's LastEventSeq should still be 0.
	pos1, err = s.keeper.GetPosition(s.ctx, pos1.Id)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), pos1.LastEventSeq,
		"first position's LastEventSeq should remain 0")
}
