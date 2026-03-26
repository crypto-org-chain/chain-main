package keeper_test

import (
	"errors"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))
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
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
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
	s.Require().ErrorIs(err, types.ErrTierNotFound)
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
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
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

// allocateRewardsToValidator funds the distribution module and allocates
// rewards to a validator so that WithdrawDelegationRewards returns them.
func (s *KeeperSuite) allocateRewardsToValidator(valAddr sdk.ValAddress, amount sdkmath.Int, denom string) {
	s.T().Helper()

	// Fund the distribution module account so it can back the allocation.
	rewardCoins := sdk.NewCoins(sdk.NewCoin(denom, amount))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, distrtypes.ModuleName, rewardCoins)
	s.Require().NoError(err)

	// Allocate through distribution so the rewards show up in WithdrawDelegationRewards.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	decRewards := sdk.NewDecCoinsFromCoins(rewardCoins...)
	err = s.app.DistrKeeper.AllocateTokensToValidator(s.ctx, val, decRewards)
	s.Require().NoError(err)
}

// setValidatorCommission overrides the genesis validator's commission rate.
// The default genesis validator has 100% commission, which means delegators
// receive nothing from AllocateTokensToValidator. This helper sets it to
// a usable rate for reward tests.
func (s *KeeperSuite) setValidatorCommission(valAddr sdk.ValAddress, rate sdkmath.LegacyDec) {
	s.T().Helper()
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	val.Commission = stakingtypes.NewCommission(rate, sdkmath.LegacyOneDec(), sdkmath.LegacyZeroDec())
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, val))
}

// fundRewardsPool funds the tier bonus rewards pool with the given amount.
func (s *KeeperSuite) fundRewardsPool(amount sdkmath.Int, denom string) {
	s.T().Helper()
	coins := sdk.NewCoins(sdk.NewCoin(denom, amount))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, coins)
	s.Require().NoError(err)
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_FirstPosition_LockTier() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create the first position with delegation — this should call
	// UpdateBaseRewardsPerShare internally. Since there's no prior
	// delegation to the validator, the ratio should be zero/empty.
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

	// First position should have empty BaseRewardsPerShare (no prior rewards).
	s.Require().True(pos.BaseRewardsPerShare.IsZero(),
		"first position should start with zero base rewards per share")
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_SecondPositionGetsUpdatedRatio_LockTier() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// The genesis validator has 100% commission — delegators get nothing.
	// Set it to 0% so all allocated rewards go to delegators.
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Create first position with same amount as initial delegation
	// Expects half the rewards to go to the tier module account
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	msg1 := &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	}
	_, err := msgServer.LockTier(s.ctx, msg1)
	s.Require().NoError(err)

	// Advance the block so the delegation's starting period in x/distribution
	// is finalized before rewards are allocated. Without this, the delegation
	// and allocation happen in the same period and WithdrawDelegationRewards
	// returns zero (startingRatio == endingRatio).
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Simulate rewards accruing by allocating through x/distribution.
	// This is the proper way — fund the distribution module and call
	// AllocateTokensToValidator so WithdrawDelegationRewards returns them.
	rewardAmount := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, rewardAmount, bondDenom)

	// Create second position — UpdateBaseRewardsPerShare should be called,
	// withdrawing from distribution and computing the ratio.
	msg2 := &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}
	_, err = msgServer.LockTier(s.ctx, msg2)
	s.Require().NoError(err)

	pos1, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	pos2, err := s.keeper.Positions.Get(s.ctx, uint64(1))
	s.Require().NoError(err)

	// First position started with zero ratio.
	s.Require().True(pos1.BaseRewardsPerShare.IsZero(),
		"first position should have zero base rewards per share")

	// Second position should have a positive ratio reflecting the reward
	// distributed across the first position's delegation shares.
	s.Require().False(pos2.BaseRewardsPerShare.IsZero(),
		"second position should have non-zero base rewards per share")

	rewardToTierModule := rewardAmount.Quo(sdkmath.NewInt(2))
	expectedRatio := sdkmath.LegacyNewDecFromInt(rewardToTierModule).Quo(pos1.DelegatedShares)

	actualRatio := pos2.BaseRewardsPerShare[0].Amount

	s.Require().True(actualRatio.Equal(expectedRatio),
		"second position ratio should equal rewardAmount / firstPositionShares, got %s want %s",
		actualRatio, expectedRatio)
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_FirstPosition_CommitDelegationToTier() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	commitAmount := val.TokensFromShares(del.Shares).TruncateInt().Quo(sdkmath.NewInt(2))

	// Create the first position with delegation — this should call
	// UpdateBaseRewardsPerShare internally. Since there's no prior
	// delegation to the validator, the ratio should be zero/empty.
	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	}

	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)

	// First position should have empty BaseRewardsPerShare (no prior rewards).
	s.Require().True(pos.BaseRewardsPerShare.IsZero(),
		"first position should start with zero base rewards per share")
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_SecondPositionGetsUpdatedRatio_CommitDelegationToTier() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// The genesis validator has 100% commission — delegators get nothing.
	// Set it to 0% so all allocated rewards go to delegators.
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Create first position with same amount as initial delegation
	// Expects half the rewards to go to the tier module account
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	msg1 := &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	}
	_, err := msgServer.LockTier(s.ctx, msg1)
	s.Require().NoError(err)

	// Advance the block so the delegation's starting period in x/distribution
	// is finalized before rewards are allocated. Without this, the delegation
	// and allocation happen in the same period and WithdrawDelegationRewards
	// returns zero (startingRatio == endingRatio).
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Simulate rewards accruing by allocating through x/distribution.
	// This is the proper way — fund the distribution module and call
	// AllocateTokensToValidator so WithdrawDelegationRewards returns them.
	rewardAmount := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, rewardAmount, bondDenom)

	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	commitAmount := val.TokensFromShares(del.Shares).TruncateInt().Quo(sdkmath.NewInt(2))

	msg := &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	}

	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().NoError(err)

	pos1, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	pos2, err := s.keeper.Positions.Get(s.ctx, uint64(1))
	s.Require().NoError(err)

	// First position started with zero ratio.
	s.Require().True(pos1.BaseRewardsPerShare.IsZero(),
		"first position should have zero base rewards per share")

	// Second position should have a positive ratio reflecting the reward
	// distributed across the first position's delegation shares.
	s.Require().False(pos2.BaseRewardsPerShare.IsZero(),
		"second position should have non-zero base rewards per share")

	rewardToTierModule := rewardAmount.Quo(sdkmath.NewInt(2))
	expectedRatio := sdkmath.LegacyNewDecFromInt(rewardToTierModule).Quo(pos1.DelegatedShares)

	actualRatio := pos2.BaseRewardsPerShare[0].Amount

	s.Require().True(actualRatio.Equal(expectedRatio),
		"second position ratio should equal rewardAmount / firstPositionShares, got %s want %s",
		actualRatio, expectedRatio)
}

// --- MsgTierDelegate tests ---

func (s *KeeperSuite) TestMsgTierDelegate_Basic() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create undelegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	// Delegate position
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().False(pos.LastBonusAccrual.IsZero(), "LastBonusAccrual should be set")
}

func (s *KeeperSuite) TestMsgTierDelegate_AlreadyDelegated() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create position with delegation
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Try to delegate again
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAlreadyDelegated)
}

// TestMsgTierDelegate_ExitingPosition verifies that a position with exit
// triggered can still be delegated per ADR-006 §10: "Lock with
// trigger_exit_immediately but no validator: Allowed. Position is created in
// exiting state. No rewards until the user delegates via MsgTierDelegate."
func (s *KeeperSuite) TestMsgTierDelegate_ExitingPosition() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Delegating an exiting position should succeed
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated(), "exiting position should now be delegated")
	s.Require().True(pos.HasTriggeredExit(), "position should still be exiting after delegation")
	s.Require().Equal(valAddr.String(), pos.Validator)
}

func (s *KeeperSuite) TestMsgTierDelegate_WrongOwner() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      wrongAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

func (s *KeeperSuite) TestMsgTierDelegate_ValidatorIndexUpdated() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	// Before delegation, no positions for this validator from the tier module
	posIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Empty(posIds)

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	// After delegation, position should appear in validator index
	posIds, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)
	s.Require().Equal(uint64(0), posIds[0])
}

// --- MsgTierUndelegate tests ---

func (s *KeeperSuite) TestMsgTierUndelegate_Basic() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create delegated + exit-triggered position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Fund the rewards pool so bonus claim doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Per ADR-006 §5.4, undelegation is allowed immediately after triggering
	// exit — no need to wait for exit commitment to elapse.
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated(), "position should not be delegated after undelegate")
	s.Require().True(pos.DelegatedShares.IsZero(), "delegated shares should be cleared")
}

func (s *KeeperSuite) TestMsgTierUndelegate_NotDelegated() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierUndelegate_ExitNotTriggered() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

// TestMsgTierUndelegate_AllowedDuringExitCommitment verifies that undelegation
// is allowed during the exit commitment period (not just after it elapses).
// Per ADR-006 §5.4, only ExitTriggeredAt != 0 is required.
func (s *KeeperSuite) TestMsgTierUndelegate_AllowedDuringExitCommitment() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Fund rewards pool so bonus claim doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Do NOT advance time — exit commitment has not elapsed yet, but
	// undelegation should still succeed because exit has been triggered.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err, "undelegation should be allowed during exit commitment period")

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())
}

func (s *KeeperSuite) TestMsgTierUndelegate_WrongOwner() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      wrongAddr.String(),
		PositionId: 0,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

func (s *KeeperSuite) TestMsgTierUndelegate_StoresUnbondingIdMapping() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Fund the rewards pool so bonus claim doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Undelegation allowed immediately after exit trigger per ADR-006 §5.4
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	// Check that at least one unbonding ID maps to position 0
	var found bool
	err = s.keeper.UnbondingIdToPositionId.Walk(s.ctx, nil, func(unbondingId, positionId uint64) (bool, error) {
		if positionId == 0 {
			found = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(found, "unbonding ID mapping should exist for position 0")
}

// --- MsgTierRedelegate tests ---

func (s *KeeperSuite) TestMsgTierRedelegate_Basic() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create a second validator to redelegate to
	dstValAddr, _ := s.createSecondValidator()

	// Create delegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	resp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   0,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(dstValAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
}

func (s *KeeperSuite) TestMsgTierRedelegate_NotDelegated() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   0,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierRedelegate_SameValidator() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create delegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   0,
		DstValidator: valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrRedelegationToSameValidator)
}

func (s *KeeperSuite) TestMsgTierRedelegate_WrongOwner() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        wrongAddr.String(),
		PositionId:   0,
		DstValidator: dstValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

func (s *KeeperSuite) TestMsgTierRedelegate_UpdatesValidatorIndex() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Position should be in source validator index
	srcIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(srcIds, 1)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   0,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	// Source validator index should be empty
	srcIds, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Empty(srcIds)

	// Destination validator index should have the position
	dstIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	s.Require().Len(dstIds, 1)
	s.Require().Equal(uint64(0), dstIds[0])
}

// --- MsgAddToTierPosition tests ---

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Undelegated() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: 0,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(sdkmath.NewInt(1500).Equal(pos.Amount), "amount should be 1500")
	s.Require().False(pos.IsDelegated())
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Delegated() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	posBefore, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: 0,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(sdkmath.NewInt(1500).Equal(posAfter.Amount), "amount should be 1500")
	s.Require().True(posAfter.DelegatedShares.GT(posBefore.DelegatedShares), "shares should increase")
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Exiting() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: 0,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrPositionExiting)
}

func (s *KeeperSuite) TestMsgAddToTierPosition_TierCloseOnly() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	// Set tier to close only
	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: 0,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgAddToTierPosition_WrongOwner() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      wrongAddr.String(),
		PositionId: 0,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

// --- MsgTriggerExitFromTier tests ---

func (s *KeeperSuite) TestMsgTriggerExitFromTier_Basic() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	resp, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().False(resp.ExitUnlockAt.IsZero())

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit())
	s.Require().Equal(resp.ExitUnlockAt, pos.ExitUnlockAt)
}

func (s *KeeperSuite) TestMsgTriggerExitFromTier_AlreadyExiting() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrPositionExiting)
}

func (s *KeeperSuite) TestMsgTriggerExitFromTier_WrongOwner() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      wrongAddr.String(),
		PositionId: 0,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

// --- MsgClaimTierRewards tests ---

func (s *KeeperSuite) TestMsgClaimTierRewards_NotDelegated() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	_, err = msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgClaimTierRewards_WrongOwner() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      wrongAddr.String(),
		PositionId: 0,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

func (s *KeeperSuite) TestMsgClaimTierRewards_Basic() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Lock an amount equal to the genesis delegation so the tier module gets a meaningful share of rewards
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Advance block and time so distribution period is finalized
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	advanceInTime := time.Hour * 24
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(advanceInTime))

	// Allocate base rewards
	baseRewardsDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, baseRewardsDistributed, bondDenom)

	// Fund the bonus rewards pool
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	expectedBonusRewards := sdkmath.LegacyNewDecFromInt(lockAmount).
		Mul(sdkmath.LegacyNewDecWithPrec(4, 2)).
		MulInt64(int64(advanceInTime / time.Second)).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()

	// amount stake is half of whats staked in total, so base rewards are half of the distributed
	expectedBaseRewards := baseRewardsDistributed.Quo(sdkmath.NewInt(2))

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().True(expectedBonusRewards.Equal(resp.BonusRewards.AmountOf(bondDenom)), "bonus rewards should be correct")
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(expectedBaseRewards.Add(expectedBonusRewards))), "owner should have received rewards matching what's expected")
}

// TestMsgAddToTierPosition_NeverDoubleClaimsRewards verifies that AddToTierPosition
// re-fetches the position after ClaimRewardsForPositions, so the updated
// BaseRewardsPerShare snapshot is not overwritten. Calling ClaimTierRewards a second
// time should yield zero base rewards if no new rewards have been allocated.
func (s *KeeperSuite) TestMsgAddToTierPosition_NeverDoubleClaimsRewards() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Allocate some base rewards before adding to position.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// AddToTierPosition internally claims rewards before delegating new tokens.
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: 0,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)

	balAfterAdd := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// No new rewards have been allocated — a second claim should yield zero.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	balAfterClaim := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Balance should not change: the first claim was already settled in AddToTierPosition.
	s.Require().Equal(balAfterAdd.Amount, balAfterClaim.Amount, "no additional rewards should be paid on second claim")
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should be zero on second claim")
	s.Require().True(resp.BonusRewards.IsZero(), "bonus rewards should also be zero on second claim: already settled in AddToTierPosition")
}

// TestMsgTierRedelegate_ClaimsRewardsBeforeRedelegating verifies that TierRedelegate
// claims pending rewards before performing the redelegation. A subsequent ClaimTierRewards
// call (with no new rewards allocated) should yield zero base rewards.
func (s *KeeperSuite) TestMsgTierRedelegate_ClaimsRewardsBeforeRedelegating() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	dstValAddr, _ := s.createSecondValidator()

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Advance time and allocate rewards so there are pending base rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierRedelegate internally claims rewards via ClaimAndRefreshPosition.
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   0,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	balAfterRedelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterRedelegate.Amount.GT(balBefore.Amount), "rewards should be paid during redelegate")

	// No new rewards allocated — subsequent ClaimTierRewards on dst validator should yield zero base.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should already be claimed during redelegate")
}

// TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating verifies that TierUndelegate
// claims pending rewards before undelegating. A subsequent ClaimTierRewards would fail
// (position no longer delegated), but the balance increase confirms rewards were paid.
func (s *KeeperSuite) TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Advance block so delegation starting period is finalized, then allocate rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 30)) // 30 days (within exit commitment)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierUndelegate internally claims rewards via ClaimAndRefreshPosition.
	// Per ADR-006 §5.4, undelegation is allowed as soon as exit is triggered.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	balAfterUndelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterUndelegate.Amount.GT(balBefore.Amount), "rewards should be paid during undelegate")
}

// TestMsgClaimTierRewards_FailsWhenBonusPoolInsufficient verifies that ClaimTierRewards
// returns ErrInsufficientBonusPool when accrued bonus cannot be paid, so the tx rolls
// back and the user can retry later without losing base rewards to a partial claim.
func (s *KeeperSuite) TestMsgClaimTierRewards_FailsWhenBonusPoolInsufficient() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Advance time and allocate base rewards, but intentionally leave bonus pool empty.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 365))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	// Bonus pool remains at 0 — bonus accrued but pool cannot cover it.

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Use a branched context so a failed message does not persist state (matches DeliverTx rollback).
	cacheCtx, _ := s.ctx.CacheContext()
	resp, err := msgServer.ClaimTierRewards(cacheCtx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().Error(err)
	s.Require().True(errors.Is(err, types.ErrInsufficientBonusPool))
	s.Require().Nil(resp)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount), "failed claim must not transfer rewards")
}

// --- MsgWithdrawFromTier tests ---

func (s *KeeperSuite) TestMsgWithdrawFromTier_Basic_Undelegated() {
	delAddr, _, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(1000)

	// Lock tokens (undelegated) with immediate exit trigger
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Advance time past exit unlock (tier exit duration is 365 days)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount),
		"response should include withdrawn amount")

	// Owner should have received the locked tokens back
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(lockAmount)),
		"owner should have received locked tokens back")

	// Position should be deleted
	_, err = s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().Error(err, "position should be deleted after withdrawal")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_PositionDeletedFromIndexes() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Verify position exists in owner index
	posIds, err := s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)

	// Verify position count for tier
	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Advance time and withdraw
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	// Owner index should be empty
	posIds, err = s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Empty(posIds, "owner index should be empty after withdrawal")

	// Position count for tier should be 0
	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count, "tier position count should be 0 after withdrawal")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_ExitNotTriggered() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock tokens without triggering exit
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotReadyToWithdraw)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_ExitCommitmentNotElapsed() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock tokens with immediate exit trigger
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Don't advance time — exit commitment hasn't elapsed
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_StillDelegated() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock with delegation and immediate exit trigger
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Advance time past exit unlock
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	// Try to withdraw while still delegated (haven't undelegated)
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrPositionStillDelegated)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_WrongOwner() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      wrongAddr.String(),
		PositionId: 0,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_PositionNotFound() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	delAddr := sdk.AccAddress([]byte("some_address________"))
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 999,
	})
	s.Require().Error(err)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_AfterUndelegate() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(1000)

	// Lock with delegation and immediate exit trigger
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Fund the rewards pool so bonus claim in undelegate doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Undelegate immediately (allowed per ADR-006 §5.4 once exit is triggered)
	undelegateResp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().False(undelegateResp.CompletionTime.IsZero())

	// Position should not be delegated but still exists
	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())

	// Advance time past exit unlock (365 days + 1 day) for withdrawal
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	// Fund the module account to simulate unbonding completion
	// (in real chain, staking end blocker would return tokens after unbonding period)
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Now withdraw — requires exit commitment elapsed
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount))

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(lockAmount)),
		"owner should have received locked tokens back after undelegate + withdraw")

	// Position should be deleted
	_, err = s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().Error(err, "position should be deleted after withdrawal")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_MultiplePositions_WithdrawOne() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create two positions with immediate exit
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(2000),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Advance time past exit
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	// Withdraw only the first position
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	// First position should be deleted
	_, err = s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().Error(err, "first position should be deleted")

	// Second position should still exist
	pos2, err := s.keeper.Positions.Get(s.ctx, uint64(1))
	s.Require().NoError(err)
	s.Require().True(sdkmath.NewInt(2000).Equal(pos2.Amount))

	// Owner should still have 1 position in index
	posIds, err := s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)
	s.Require().Equal(uint64(1), posIds[0])

	// Tier should have 1 position remaining
	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)
}

// --- ADR-006 Deviation Fix Tests ---

// TestMsgTierDelegate_ExitingPosition_ThenEarnRewards verifies the full ADR-006
// §10 flow: lock with trigger_exit_immediately but no validator, then delegate
// later, and confirm bonus rewards accrue until ExitUnlockTime.
func (s *KeeperSuite) TestMsgTierDelegate_ExitingPosition_ThenEarnRewards() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		TriggerExitImmediately: true, // exit triggered, no validator
	})
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit(), "position should be exiting")
	s.Require().False(pos.IsDelegated(), "position should not be delegated yet")

	// Delegate the exiting position to earn rewards
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated(), "position should now be delegated")
	s.Require().True(pos.HasTriggeredExit(), "position should still be exiting")
	s.Require().False(pos.LastBonusAccrual.IsZero(), "LastBonusAccrual should be set")

	// Advance time and allocate rewards
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 30)) // 30 days
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Claim rewards — should succeed and pay both base + bonus
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().False(resp.BaseRewards.IsZero() && resp.BonusRewards.IsZero(),
		"exiting-then-delegated position should earn rewards")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount),
		"rewards should have been transferred to owner")
}

// TestMsgTierUndelegate_ImmediatelyAfterExit verifies that undelegation
// succeeds immediately after triggering exit, without waiting for the full
// exit commitment to elapse. This is the ADR-006 §5.4 behavior.
func (s *KeeperSuite) TestMsgTierUndelegate_ImmediatelyAfterExit() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create delegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Trigger exit
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Undelegate immediately — should succeed without time advance
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err, "undelegation should succeed immediately after exit trigger")
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())
	s.Require().True(pos.HasTriggeredExit())

	// Withdrawal should still require exit commitment to elapse
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached,
		"withdrawal should still require exit commitment to elapse")
}

// TestMsgTierUndelegate_FullLifecycle_EarlyUndelegate tests the complete flow:
// lock → delegate → trigger exit → undelegate immediately → wait for exit →
// withdraw. This verifies the ADR-006 intended timeline where unbonding
// overlaps with exit commitment.
func (s *KeeperSuite) TestMsgTierUndelegate_FullLifecycle_EarlyUndelegate() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(1000)

	// Lock with delegation and immediate exit
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Undelegate immediately (ADR-006 §5.4)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	// Cannot withdraw yet — exit commitment hasn't elapsed
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)

	// Advance time past exit commitment (365 days + 1 day)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	// Fund module account to simulate unbonding completion
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Now withdraw should succeed
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount))

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(lockAmount)))

	// Position should be deleted
	_, err = s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().Error(err, "position should be deleted after withdrawal")
}

// --- Event emission tests ---

// TestMsgClaimTierRewards_EmitsEvent verifies that MsgClaimTierRewards emits
// EventTierRewardsClaimed after a successful claim.
func (s *KeeperSuite) TestMsgClaimTierRewards_EmitsEvent() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Isolate events from this specific call.
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.ClaimTierRewards(freshCtx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	found := false
	for _, e := range freshCtx.EventManager().Events() {
		if e.Type == "chainmain.tieredrewards.v1.EventTierRewardsClaimed" {
			found = true
			break
		}
	}
	s.Require().True(found, "EventTierRewardsClaimed should be emitted by ClaimTierRewards")
}

// TestMsgAddToTierPosition_EmitsBonusRewardsClaimedSideEffect verifies that
// AddToTierPosition on a delegated position emits EventBonusRewardsClaimed as
// a side effect of the implicit reward claim before adding tokens.
// This covers the skipped integration test test_add_to_position_reward_side_effect.
func (s *KeeperSuite) TestMsgAddToTierPosition_EmitsBonusRewardsClaimedSideEffect() {
	addr, _, pos := s.setupPositionForBonusTest()

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Advance time so bonus accrues (LastBonusAccrual is initialized at position creation by WithDelegation).
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Fund addr for the add operation; setupPositionForBonusTest exhausted its initial balance.
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))))
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.AddToTierPosition(freshCtx, &types.MsgAddToTierPosition{
		Owner:      addr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	found := false
	for _, e := range freshCtx.EventManager().Events() {
		if e.Type == "chainmain.tieredrewards.v1.EventBonusRewardsClaimed" {
			found = true
			break
		}
	}
	s.Require().True(found, "EventBonusRewardsClaimed should be emitted as side effect of AddToTierPosition on a delegated position")
}
