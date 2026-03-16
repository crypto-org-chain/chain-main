package keeper_test

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
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
