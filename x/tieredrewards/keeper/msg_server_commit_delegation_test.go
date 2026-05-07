package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
	pos, err := s.keeper.GetPositionState(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().Equal(delAddr.String(), pos.Owner)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Delegation.ValidatorAddress)
	s.Require().True(pos.Delegation.Shares.Equal(halfShares))
	s.Require().Equal(uint64(0), pos.LastEventSeq, "LastEventSeq should be 0 for fresh validator")

	// The position's delegator address holds the delegation.
	posDelAddr := types.GetDelegatorAddress(pos.Id)
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(posDel.Shares.Equal(halfShares))

	// Verify that the distribution rewards for this delegation are routed to the owner.
	withdrawAddr, err := s.app.DistrKeeper.GetDelegatorWithdrawAddr(s.ctx, posDelAddr)
	s.Require().NoError(err)
	s.Require().Equal(delAddr.String(), withdrawAddr.String(), "withdraw addr should route to owner")

	valCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount)
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
	pos, err := s.keeper.GetPositionState(s.ctx, uint64(0))
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

	pos, err := s.keeper.GetPositionState(s.ctx, uint64(0))
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
	valAddr := sdk.MustValAddressFromBech32(pos1.Delegation.ValidatorAddress)

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

	pos2, err := s.keeper.GetPositionState(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// The new position's LastEventSeq should equal 1 (the slash event).
	s.Require().Equal(uint64(1), pos2.LastEventSeq,
		"new position should skip prior events; LastEventSeq should be 1")

	// The first position's LastEventSeq should still be 0.
	pos1, err = s.keeper.GetPositionState(s.ctx, pos1.Id)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), pos1.LastEventSeq,
		"first position's LastEventSeq should remain 0")
}

// TestMsgCommitDelegationToTier_ExactlyFundedOwner tests that it is not required
// for the owner to hold extra funds when transferring a delegation to a tier position.
func (s *KeeperSuite) TestMsgCommitDelegationToTier_ExactlyFundedOwner() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	commitAmount := sdkmath.NewInt(1_000_000)

	owner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, owner,
		sdk.NewCoins(sdk.NewCoin(bondDenom, commitAmount))))

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, owner, commitAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	// Owner has no liquid bond denom left — everything is in the delegation.
	s.Require().True(s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom).Amount.IsZero(),
		"owner should have 0 liquid bondDenom after fully delegating")

	_, err = msgServer.CommitDelegationToTier(s.ctx, &types.MsgCommitDelegationToTier{
		DelegatorAddress: owner.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	})
	s.Require().NoError(err, "commit must not require liquid funds on top of the delegation")

	posDelAddr := types.GetDelegatorAddress(0)
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(posDel.Shares.IsPositive(), "position's delegator account must hold the transferred delegation")

	// Owner's original delegation must be fully consumed.
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, owner, valAddr)
	s.Require().Error(err, "owner should no longer hold a delegation at this validator")

	// Position's delegation token value should equal the committed amount
	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	posDelTokens := valAfter.TokensFromShares(posDel.Shares).TruncateInt()
	s.Require().True(posDelTokens.Equal(commitAmount),
		"position's delegation should equal committed amount: got %s want %s", posDelTokens, commitAmount)

	// No liquid coins stranded on the position's delegator account (the old bug left msg.Amount there).
	s.Require().True(s.app.BankKeeper.GetBalance(s.ctx, posDelAddr, bondDenom).Amount.IsZero(),
		"position's delegator account must not hold liquid bondDenom after commit")
}

// TestMsgCommitDelegationToTier_CreatesDelegatorAuthAccount verifies that
// CommitDelegationToTier explicitly creates a BaseAccount for the position's
// delegator address.
func (s *KeeperSuite) TestMsgCommitDelegationToTier_CreatesDelegatorAuthAccount() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	commitAmount := sdkmath.NewInt(1_000_000)
	owner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, owner,
		sdk.NewCoins(sdk.NewCoin(bondDenom, commitAmount))))

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, owner, commitAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	// Predict the next position's delegator address and confirm no auth
	// account exists yet — if one did, the commit path's idempotent guard
	// would mask the behavior we want to verify.
	nextId, err := s.keeper.NextPositionId.Peek(s.ctx)
	s.Require().NoError(err)
	predictedDelAddr := types.GetDelegatorAddress(nextId)
	s.Require().Nil(s.app.AccountKeeper.GetAccount(s.ctx, predictedDelAddr),
		"auth account must not exist before commit")

	resp, err := msgServer.CommitDelegationToTier(s.ctx, &types.MsgCommitDelegationToTier{
		DelegatorAddress: owner.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	})
	s.Require().NoError(err)
	s.Require().Equal(nextId, resp.PositionId)

	// After commit the auth account for the position's delegator address must
	// exist; otherwise any later undelegation + CompleteUnbonding would fail
	// silently in the staking EndBlocker.
	acc := s.app.AccountKeeper.GetAccount(s.ctx, predictedDelAddr)
	s.Require().NotNil(acc, "auth account must be created by CommitDelegationToTier")
	s.Require().Equal(predictedDelAddr, acc.GetAddress())
}

func (s *KeeperSuite) TestMsgCommitDelegationToTier_ValidatorNotBonded() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.jailAndUnbondValidator(valAddr)

	_, err := msgServer.CommitDelegationToTier(s.ctx, &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
	})
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)
}
