package keeper_test

import (
	"errors"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// --- MsgLockTier tests ---

func (s *KeeperSuite) TestMsgLockTier_Basic() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
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
	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().Equal(freshAddr.String(), pos.Owner)
	s.Require().True(sdkmath.NewInt(1000).Equal(pos.Amount))
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))
}

func (s *KeeperSuite) TestMsgLockTier_WithImmediateTriggerExit() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
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

	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
}

func (s *KeeperSuite) TestMsgLockTier_TierNotFound() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               999,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	_, err = msgServer.LockTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTierNotFound)
}

func (s *KeeperSuite) TestMsgLockTier_TierCloseOnly() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set tier to close only
	tier := newTestTier(1)
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
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(999))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(999), // min is 1000
		ValidatorAddress: valAddr.String(),
	}

	_, err = msgServer.LockTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrMinLockAmountNotMet)
}

func (s *KeeperSuite) TestMsgLockTier_TransfersTokens() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(1000))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, freshAddr, bondDenom)

	msg := &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	}

	_, err = msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, freshAddr, bondDenom)
	s.Require().Equal(sdkmath.NewInt(1000), balBefore.Amount.Sub(balAfter.Amount))
}

// --- MsgCommitDelegationToTier tests ---

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

	// Module should have delegation on the same validator
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	moduleDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, moduleAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(moduleDel.Shares.Equal(halfShares))
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

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_FirstPosition_LockTier() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	_, bondDenom := s.getStakingData()
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, sdk.MustAccAddressFromBech32(pos.Owner), sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create the first position with delegation — this should call
	// UpdateBaseRewardsPerShare internally. Since there's no prior
	// delegation to the validator, the ratio should be zero/empty.
	msg := &types.MsgLockTier{
		Owner:            pos.Owner,
		Id:               1,
		Amount:           lockAmt,
		ValidatorAddress: valAddr.String(),
	}

	resp, err := msgServer.LockTier(s.ctx, msg)
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// First position should have empty BaseRewardsPerShare (no prior rewards).
	s.Require().True(pos.BaseRewardsPerShare.IsZero(),
		"first position should start with zero base rewards per share")
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_SecondPositionGetsUpdatedRatio_LockTier() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	// The genesis validator has 100% commission — delegators get nothing.
	// Set it to 0% so all allocated rewards go to delegators.
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

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
	lockAmt2 := sdkmath.NewInt(1000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	msg2 := &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmt2,
		ValidatorAddress: valAddr.String(),
	}
	_, err = msgServer.LockTier(s.ctx, msg2)
	s.Require().NoError(err)

	pos1, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos2, err := s.keeper.GetPosition(s.ctx, uint64(1))
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
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
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

	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)

	// First position should have empty BaseRewardsPerShare (no prior rewards).
	s.Require().True(pos.BaseRewardsPerShare.IsZero(),
		"first position should start with zero base rewards per share")
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_SecondPositionGetsUpdatedRatio_CommitDelegationToTier() {
	// Create first position with same amount as initial delegation
	// Expects half the rewards to go to the tier module account
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPositionWithDelegator(lockAmount, false)
	_, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// The genesis validator has 100% commission — delegators get nothing.
	// Set it to 0% so all allocated rewards go to delegators.
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

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
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.CommitDelegationToTier(s.ctx, msg)
	s.Require().NoError(err)

	pos1, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	pos2, err := s.keeper.GetPosition(s.ctx, uint64(1))
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
	// Create delegated position
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redelegation slash zeroing the position
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Add funds back
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     lockAmt,
	})
	s.Require().NoError(err)

	// Delegate position
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().False(pos.LastBonusAccrual.IsZero(), "LastBonusAccrual should be set")
}

func (s *KeeperSuite) TestMsgTierDelegate_AlreadyDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create position with delegation

	// Try to delegate again
	_, err := msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAlreadyDelegated)
}

// TestMsgTierDelegate_AmountZero verifies that TierDelegate is rejected on a
// zero-amount position
func (s *KeeperSuite) TestMsgTierDelegate_AmountZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash clearing delegation and zeros amount
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)
}

// TestMsgTierDelegate_AmountZero_TriggeredExit verifies that TierDelegate is rejected on a
// zero-amount position with exit triggered.
func (s *KeeperSuite) TestMsgTierDelegate_AmountZero_ExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash clearing delegation and zeros amount
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)
}

// TestMsgTierDelegate_ExitInProgress verifies that TierDelegate succeeds when
// exit is triggered but not yet elapsed on an undelegated position.
func (s *KeeperSuite) TestMsgTierDelegate_ExitInProgress() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redelegation slash zeroing out the position
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Add funds back
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	// Trigger exit
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	// Delegate while exit is in progress
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))
}

// TestMsgTierDelegate_ExitElapsed verifies that TierDelegate is rejected when
// exit has fully elapsed — user must ClearPosition first, or undelegate and withdraw.
func (s *KeeperSuite) TestMsgTierDelegate_ExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance past exit duration, then undelegate + complete unbonding.
	s.advancePastExitDuration()
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.completeStakingUnbonding(valAddr)

	// Delegate after exit elapsed — should be rejected.
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationElapsed)
}

func (s *KeeperSuite) TestMsgTierDelegate_WrongOwner() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redeleg slash to get undelegated position without exit.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgTierDelegate_ValidatorIndexUpdated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redeleg slash to get undelegated position.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	// After delegation, position should appear in validator index
	posIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)
	s.Require().Equal(uint64(0), posIds[0])
}

// --- MsgTierUndelegate tests ---

func (s *KeeperSuite) TestMsgTierUndelegate_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create delegated + exit-triggered position

	// Fund the rewards pool so bonus claim doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	s.advancePastExitDuration()
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().False(pos.IsDelegated(), "position should not be delegated after undelegate")
	s.Require().True(pos.DelegatedShares.IsZero(), "delegated shares should be cleared")

	// Verify redelegation unbonding ID was written to UnbondingDelegationMappings, not RedelegationMappings.
	var unbondingFound bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			unbondingFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(unbondingFound, "undelegation unbonding ID should be in UnbondingDelegationMappings")

	var redelegationFound bool
	err = s.keeper.RedelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			redelegationFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().False(redelegationFound, "undelegation unbonding ID should not be stored in RedelegationMappings")
}

func (s *KeeperSuite) TestMsgTierUndelegate_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// First undelegate succeeds
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Second undelegate should fail with ErrPositionNotDelegated
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierUndelegate_ExitNotTriggered() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

// TestMsgTierUndelegate_ExitDurationNotReached verifies that TierUndelegate is
// rejected when exit is triggered but duration has not elapsed.
func (s *KeeperSuite) TestMsgTierUndelegate_ExitDurationNotReached() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgTierUndelegate_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	s.advancePastExitDuration()
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// --- MsgTierRedelegate tests ---

func (s *KeeperSuite) TestMsgTierRedelegate_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create a second validator to redelegate to
	dstValAddr, _ := s.createSecondValidator()
	// Create delegated position

	resp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(dstValAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())

	// Verify redelegation unbonding ID was written to RedelegationMappings, not UnbondingDelegationMappings.
	var redelegationFound bool
	err = s.keeper.RedelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			redelegationFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(redelegationFound, "redelegation unbonding ID should be stored in RedelegationMappings")

	var unbondingFound bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			unbondingFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().False(unbondingFound, "redelegation unbonding ID should NOT be in UnbondingDelegationMappings")
}

func (s *KeeperSuite) TestMsgTierRedelegate_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Undelegate first so position is not delegated
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierRedelegate_SameValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create delegated position

	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrRedelegationToSameValidator)
}

// TestMsgTierRedelegate_AmountZero verifies that TierRedelegate is rejected on a
// zero-amount bonded position (slash from bonded validator)
func (s *KeeperSuite) TestMsgTierRedelegate_AmountZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate slash by zeroing amount
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	dstValidator := sdk.ValAddress([]byte("dst_validator________"))
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValidator.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)
}

// TestMsgTierRedelegate_ExitInProgress verifies that TierRedelegate succeeds
// when exit is in progress.
func (s *KeeperSuite) TestMsgTierRedelegate_ExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Exit is triggered but NOT elapsed.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(dstValAddr.String(), pos.Validator)
	s.Require().True(pos.HasTriggeredExit())
}

// TestMsgTierRedelegate_ExitElapsed verifies that TierRedelegate is rejected
// when exit has fully elapsed — user must ClearPosition first.
func (s *KeeperSuite) TestMsgTierRedelegate_ExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.advancePastExitDuration()
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Exit has elapsed, position still delegated.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationElapsed)
}

func (s *KeeperSuite) TestMsgTierRedelegate_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        wrongAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgTierRedelegate_UpdatesValidatorIndex() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Position should be in source validator index
	srcIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(srcIds, 1)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
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
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Simulate redelegation slash clearing delegation and zeroing amount.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, bondDenom := s.getStakingData()
	addPosAmt := sdkmath.NewInt(500)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addPosAmt,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(addPosAmt.Equal(pos.Amount), "amount should be 500 (added to zero)")
	s.Require().False(pos.IsDelegated(), "position should remain undelegated")
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Delegated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	addPosAmt := sdkmath.NewInt(500)
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(sdkmath.NewInt(1500).Equal(posAfter.Amount), "amount should be 1500")
	s.Require().True(posAfter.DelegatedShares.GT(posBefore.DelegatedShares), "shares should increase")
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Exiting() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrPositionTriggeredExit)
}

func (s *KeeperSuite) TestMsgAddToTierPosition_TierCloseOnly() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set tier to close only
	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err := msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgAddToTierPosition_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// --- MsgTriggerExitFromTier tests ---

func (s *KeeperSuite) TestMsgTriggerExitFromTier_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	resp, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.ExitUnlockAt.IsZero())

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit())
	s.Require().Equal(resp.ExitUnlockAt, pos.ExitUnlockAt)
}

func (s *KeeperSuite) TestMsgTriggerExitFromTier_AlreadyExiting() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionTriggeredExit)
}

func (s *KeeperSuite) TestMsgTriggerExitFromTier_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// --- MsgClearPosition tests ---

func (s *KeeperSuite) TestMsgClearPosition_ClearsExitAndAllowsAddToTier() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	addPosAmt := sdkmath.NewInt(500)
	_, bondDenom := s.getStakingData()
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrPositionTriggeredExit)

	clearResp, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), clearResp.PositionId)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit())

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)
}

// TestMsgClearPosition_UpdatesLastBonusAccrualAfterExitElapsed verifies that
// ClearPosition past exit duration updates LastBonusAccrual to the current
// block time, confirming reward settlement occurred.
func (s *KeeperSuite) TestMsgClearPosition_UpdatesLastBonusAccrualAfterExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Advance past exit duration
	s.advancePastExitDuration()

	_, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit())
	s.Require().Equal(s.ctx.BlockTime(), pos.LastBonusAccrual,
		"last_bonus_accrual should equal block time after ClearPosition")
}

func (s *KeeperSuite) TestMsgClearPosition_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgClearPosition_NoOpWhenNotExiting() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Create claimable rewards and advance bonus accrual time to catch unintended side-effects.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000), bondDenom)

	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Isolate events from this specific call.
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.ClearPosition(freshCtx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	s.Require().False(pos.HasTriggeredExit(), "position should still not be exiting")
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount), "clearing a non-exiting position must not claim rewards")
	s.Require().Equal(posBefore.LastBonusAccrual, pos.LastBonusAccrual, "clearing a non-exiting position must not mutate accrual state")

	foundExitCleared := false
	for _, e := range freshCtx.EventManager().Events() {
		if e.Type == "chainmain.tieredrewards.v1.EventExitCleared" {
			foundExitCleared = true
			break
		}
	}
	s.Require().False(foundExitCleared, "clearing a non-exiting position must not emit EventExitCleared")
}

// TestMsgClearPosition_RejectsWhileUnbonding verifies that ClearPosition is
// rejected when the position is unbonding.
func (s *KeeperSuite) TestMsgClearPosition_RejectsWhileUnbonding() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	// Position is still unbonding
	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionUnbonding)
}

// TestMsgClearPosition_DelegatedPastExitSettlesBonusBeforeClear verifies that clearing
// exit on a delegated position past exit_unlock_at succeeds and runs reward settlement
// first (bonus capped at exit_unlock_at), avoiding an exploitable bonus pool window.
func (s *KeeperSuite) TestMsgClearPosition_DelegatedPastExitSettlesBonusBeforeClear() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	// Past exit commitment (365d) plus one day — bonus must not accrue past unlock without clearing.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	_, err := msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit())
}

// TestMsgClearPosition_RejectsAfterUnbondingCompleted verifies that ClearPosition
// is rejected on an undelegated position after unbonding completes.
func (s *KeeperSuite) TestMsgClearPosition_RejectsAfterUnbondingCompleted() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr)

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

// TestMsgClearPosition_AllowsPendingRedelegationWhenStillDelegated verifies that
// ClearPosition can clear exit after exit elapsed even if the staking-layer
// redelegation is still pending, so long as the position remains delegated.
func (s *KeeperSuite) TestMsgClearPosition_AllowsPendingRedelegationWhenStillDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	redelegationIter, err := s.keeper.RedelegationMappings.Indexes.ByPosition.MatchExact(s.ctx, uint64(0))
	s.Require().NoError(err)
	redelegationIDs, err := redelegationIter.PrimaryKeys()
	s.Require().NoError(err)
	s.Require().NotEmpty(redelegationIDs, "redelegation mapping should exist after TierRedelegate")

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.HasTriggeredExit(), "clear should reset exit state")
	s.Require().True(pos.IsDelegated(), "position should remain delegated on the destination validator")
	s.Require().Equal(dstValAddr.String(), pos.Validator)

	redelegationIter, err = s.keeper.RedelegationMappings.Indexes.ByPosition.MatchExact(s.ctx, uint64(0))
	s.Require().NoError(err)
	redelegationIDs, err = redelegationIter.PrimaryKeys()
	s.Require().NoError(err)
	s.Require().NotEmpty(redelegationIDs, "clearing exit should not delete pending redelegation tracking")
}

// TestClearPositionAfterRedelegationSlashAllSharesBurnt verifies
// ClearPosition remains blocked after exit elapsed when a redelegation slash
// burns all shares and clears delegation while redelegation mapping is active.
func (s *KeeperSuite) TestClearPositionAfterRedelegationSlashAllSharesBurnt() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	redelegateResp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	posBeforeSlash, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posBeforeSlash.DelegatedShares.IsPositive(), "test setup failed: expected delegated shares before slash")
	s.Require().True(posBeforeSlash.IsDelegated(), "test setup failed: position should be delegated before slash")

	// Burn all shares through redelegation slash callback.
	shareBurnt := posBeforeSlash.DelegatedShares.Add(sdkmath.LegacyOneDec())
	err = s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, redelegateResp.UnbondingId, posBeforeSlash.Amount, shareBurnt)
	s.Require().NoError(err)

	posAfterSlash, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfterSlash.IsDelegated(), "delegation should be cleared when all shares are burnt")
	s.Require().True(posAfterSlash.DelegatedShares.IsZero(), "delegated shares should be zero after full share burn")
	s.Require().True(posAfterSlash.Amount.IsZero(), "amount should be zero after full share burn")
	s.Require().True(posAfterSlash.HasTriggeredExit(), "slash should not clear exit trigger")

	// Redelegation mapping stays active here, but the clear failure reason is that
	// the slash has already cleared delegation from the position.
	redelegationIter, err := s.keeper.RedelegationMappings.Indexes.ByPosition.MatchExact(s.ctx, pos.Id)
	s.Require().NoError(err)
	redelegationIDs, err := redelegationIter.PrimaryKeys()
	s.Require().NoError(err)
	s.Require().NotEmpty(redelegationIDs, "redelegation mapping should remain active for this corner case")

	s.advancePastExitDuration()

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)

	posAfterClearAttempt, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfterClearAttempt.HasTriggeredExit(), "failed clear attempt should not reset exit state")
	s.Require().False(posAfterClearAttempt.IsDelegated(), "failed clear attempt should keep cleared delegation state")
}

// TestExitTriggerClearCycles_BonusAccrualCorrectness verifies that
// repeated TriggerExit/ClearPosition cycles do not double-count or under-count
// bonus accrual when cycle durations are identical.
func (s *KeeperSuite) TestExitTriggerClearCycles_BonusAccrualCorrectness() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	cycleDuration := 24 * time.Hour

	balBeforeCycle1 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(cycleDuration))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfterCycle1, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfterCycle1.HasTriggeredExit(), "clear should reset exit state")
	s.Require().True(posAfterCycle1.IsDelegated(), "clear cycle should keep delegated position active")
	s.Require().Equal(s.ctx.BlockTime(), posAfterCycle1.LastBonusAccrual, "clear should checkpoint bonus accrual at current time")

	balAfterCycle1 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	cycle1Payout := balAfterCycle1.Amount.Sub(balBeforeCycle1.Amount)
	s.Require().True(cycle1Payout.IsPositive(), "first cycle should pay positive bonus")

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(cycleDuration))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfterCycle2, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfterCycle2.HasTriggeredExit(), "clear should reset exit state in repeated cycle")
	s.Require().True(posAfterCycle2.IsDelegated(), "repeated clear should keep delegated position active")
	s.Require().Equal(s.ctx.BlockTime(), posAfterCycle2.LastBonusAccrual, "repeated clear should checkpoint to current time")

	balAfterCycle2 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	cycle2Payout := balAfterCycle2.Amount.Sub(balAfterCycle1.Amount)
	s.Require().True(cycle2Payout.IsPositive(), "second cycle should pay positive bonus")
	s.Require().True(cycle2Payout.Equal(cycle1Payout),
		"equal-duration cycles should pay equal bonus, got cycle1=%s cycle2=%s", cycle1Payout, cycle2Payout)
}

// --- MsgClaimTierRewards tests ---

func (s *KeeperSuite) TestMsgClaimTierRewards_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Undelegate so the position is not delegated
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should be zero when not delegated")
	s.Require().True(resp.BonusRewards.IsZero(), "bonus rewards should be zero when not delegated")
}

func (s *KeeperSuite) TestMsgClaimTierRewards_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgClaimTierRewards_Basic() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	// Lock an amount equal to the genesis delegation so the tier module gets a meaningful share of rewards

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
		PositionId: pos.Id,
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
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Allocate some base rewards before adding to position.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	addPosAmt := sdkmath.NewInt(500)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)

	// AddToTierPosition internally claims rewards before delegating new tokens.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addPosAmt,
	})
	s.Require().NoError(err)

	balAfterAdd := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// No new rewards have been allocated — a second claim should yield zero.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
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
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	dstValAddr, _ := s.createSecondValidator()

	// Advance time and allocate rewards so there are pending base rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierRedelegate internally claims rewards.
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	balAfterRedelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterRedelegate.Amount.GT(balBefore.Amount), "rewards should be paid during redelegate")

	// No new rewards allocated — subsequent ClaimTierRewards on dst validator should yield zero base.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should already be claimed during redelegate")
}

// TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating verifies that TierUndelegate
// claims pending rewards before undelegating. A subsequent ClaimTierRewards would fail
// (position no longer delegated), but the balance increase confirms rewards were paid.
func (s *KeeperSuite) TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance block so delegation starting period is finalized, then allocate rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.advancePastExitDuration()
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierUndelegate internally claims rewards.
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	balAfterUndelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterUndelegate.Amount.GT(balBefore.Amount), "rewards should be paid during undelegate")
}

// TestMsgClaimTierRewards_FailsWhenBonusPoolInsufficient verifies that ClaimTierRewards
// returns ErrInsufficientBonusPool when accrued bonus cannot be paid, so the tx rolls
// back and the user can retry later without losing base rewards to a partial claim.
func (s *KeeperSuite) TestMsgClaimTierRewards_FailsWhenBonusPoolInsufficient() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

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
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().True(errors.Is(err, types.ErrInsufficientBonusPool))
	s.Require().Nil(resp)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount), "failed claim must not transfer rewards")
}

// --- MsgWithdrawFromTier tests ---

func (s *KeeperSuite) TestMsgWithdrawFromTier_Basic_Undelegated() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Lock tokens with delegation and immediate exit trigger

	// Advance time past exit unlock (tier exit duration is 365 days)
	s.advancePastExitDuration()

	// Fund bonus pool before undelegation
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Undelegate first
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Advance time past staking unbonding period and complete unbonding so
	// the staking module returns tokens to the tier module account.
	s.completeStakingUnbonding(valAddr)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
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
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_PositionDeletedFromIndexes() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Verify position exists in owner index
	posIds, err := s.keeper.GetPositionsIdsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)

	// Verify position count for tier
	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Advance time and undelegate
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Advance time past staking unbonding period and complete unbonding so
	// the staking module returns tokens to the tier module account.
	s.completeStakingUnbonding(valAddr)

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
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
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Lock tokens without triggering exit

	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_ExitCommitmentNotElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Lock tokens with immediate exit trigger

	// Don't advance time — exit commitment hasn't elapsed
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_StillDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Lock with delegation and immediate exit trigger

	// Advance time past exit unlock
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	// Try to withdraw while still delegated (haven't undelegated)
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionDelegated)
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
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
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Lock with delegation and immediate exit trigger

	// Fund the rewards pool so bonus claim in undelegate doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Advance past exit duration so undelegation is allowed.
	s.advancePastExitDuration()
	undelegateResp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(undelegateResp.CompletionTime.IsZero())

	var mappingExistsBeforeWithdraw bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, positionId uint64) (bool, error) {
		if positionId == 0 {
			mappingExistsBeforeWithdraw = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(mappingExistsBeforeWithdraw, "unbonding mapping should exist before withdrawal")

	// Position should not be delegated but still exists
	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())

	// Advance time past exit unlock (365 days + 1 day) for withdrawal
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 366))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Now withdraw — requires exit commitment elapsed and unbonding completed
	s.completeStakingUnbonding(valAddr)
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.Amount.AmountOf(bondDenom).Equal(lockAmount))

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(lockAmount)),
		"owner should have received locked tokens back after undelegate + withdraw")

	// Position should be deleted
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position should be deleted after withdrawal")

	var mappingExistsAfterWithdraw bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, positionId uint64) (bool, error) {
		if positionId == 0 {
			mappingExistsAfterWithdraw = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().False(mappingExistsAfterWithdraw, "unbonding mapping should be cleaned after position deletion")
}

func (s *KeeperSuite) TestMsgWithdrawFromTier_MultiplePositions_WithdrawOne() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	// Create two positions with immediate exit
	lockAmt2 := sdkmath.NewInt(2000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmt2,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Advance time past exit
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Undelegate both positions
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 1,
	})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr)

	// Withdraw only the first position
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// First position should be deleted
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "first position should be deleted")

	// Second position should still exist
	pos2, err := s.keeper.GetPosition(s.ctx, uint64(1))
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
// §10 flow: lock with trigger_exit_immediately and validator, and confirm
// bonus rewards accrue until ExitUnlockTime.
func (s *KeeperSuite) TestMsgTierDelegate_ExitingPosition_ThenEarnRewards() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit(), "position should be exiting")
	s.Require().True(pos.IsDelegated(), "position should be delegated")
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
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.BaseRewards.IsZero() && resp.BonusRewards.IsZero(),
		"exiting-then-delegated position should earn rewards")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount),
		"rewards should have been transferred to owner")
}

// --- Event emission tests ---

// TestMsgClaimTierRewards_EmitsEvent verifies that MsgClaimTierRewards emits
// EventTierRewardsClaimed after a successful claim.
func (s *KeeperSuite) TestMsgClaimTierRewards_EmitsEvent() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Isolate events from this specific call.
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err := msgServer.ClaimTierRewards(freshCtx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
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
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	_, bondDenom := s.getStakingData()
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Advance time so bonus accrues (LastBonusAccrual is initialized at position creation by WithDelegation).
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	addToPosAmt := sdkmath.NewInt(1000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addToPosAmt)))
	s.Require().NoError(err)
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.AddToTierPosition(freshCtx, &types.MsgAddToTierPosition{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     addToPosAmt,
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

// TestMsgTierUndelegate_ReconcilesAmount: after TierUndelegate,
// pos.Amount is reconciled with the actual token return value from the SDK's
// share→token conversion, preventing insolvency on later withdrawal.
func (s *KeeperSuite) TestMsgTierUndelegate_ReconcilesAmount() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash the validator FIRST to create a non-1:1 exchange rate so that
	// share→token conversion actually truncates.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%

	lockAmount := sdkmath.NewInt(10001) // odd number to maximize truncation
	addr := s.fundRandomAddr(bondDenom, lockAmount)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  addr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos := positions[0]
	s.Require().True(pos.IsDelegated())

	// Compute what the SDK will actually return when converting shares→tokens.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedReturn := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	// At the 0.9 exchange rate, round-trip truncation loses 1 token.
	s.Require().Equal(sdkmath.NewInt(10000).String(), expectedReturn.String(),
		"expected return should be 10000 (1 token lost to truncation)")

	s.fundRewardsPool(sdkmath.NewInt(2_000_000), bondDenom)

	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// pos.Amount must equal the actual return value, not the original lockAmount.
	s.Require().Equal(expectedReturn.String(), pos.Amount.String(),
		"pos.Amount must equal actual return value")
	s.Require().Equal(sdkmath.NewInt(10000).String(), pos.Amount.String(),
		"pos.Amount should be exactly 10000 after reconciliation")
}

// TestMsgTierUndelegate_ReconcilesAmountUpward verifies that TierUndelegate
// trusts the staking module's exact return amount even when stored position
// accounting is stale and too low.
func (s *KeeperSuite) TestMsgTierUndelegate_ReconcilesAmountUpward() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]
	s.Require().True(pos.IsDelegated())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedReturn := val.TokensFromShares(pos.DelegatedShares).TruncateInt()
	s.Require().Equal(lockAmount.String(), expectedReturn.String(),
		"test setup expects a 1:1 validator exchange rate")

	// Seed a stale underestimated amount to verify undelegation overwrites it
	// with the staking module's authoritative return amount.
	pos.UpdateAmount(expectedReturn.SubRaw(1))
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(expectedReturn.String(), pos.Amount.String(),
		"pos.Amount must be overwritten with the SDK return amount")

	s.completeStakingUnbonding(valAddr)

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(expectedReturn.String(), resp.Amount.AmountOf(bondDenom).String(),
		"withdrawn amount should equal the SDK return amount")
}

func (s *KeeperSuite) TestMsgTierUndelegate_AfterBondedSlash_Succeeds() {
	lockAmount := sdkmath.NewInt(10_000)
	pos:= s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(1, 2)) // 1%

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.advancePastExitDuration()

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero(), "undelegation should still succeed after a bonded slash")
}

// TestMsgAddToTierPosition_ReconcilesAmountWithShares: after
// AddToTierPosition on a delegated position, pos.Amount matches the actual
// token value from total shares, not the arithmetic sum of deposits.
func (s *KeeperSuite) TestMsgAddToTierPosition_ReconcilesAmountWithShares() {
	lockAmount := sdkmath.NewInt(10001)
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Slash the validator to create a non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	addAmount := sdkmath.NewInt(5001)
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      addr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// pos.Amount must equal what the validator says the total shares are worth.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from shares")

	// The reconciled amount must be strictly less than the arithmetic sum
	// because the non-1:1 exchange rate causes truncation loss.
	arithmeticSum := lockAmount.Add(addAmount) // 15002
	s.Require().NotEqual(arithmeticSum.String(), pos.Amount.String(),
		"pos.Amount must differ from naive arithmetic sum due to truncation")
}

// TestMsgAddToTierPosition_MultipleCalls_NoDivergence verifies that repeated
// AddToTierPosition calls don't compound rounding divergence. After N calls,
// pos.Amount still matches TokensFromShares(totalShares).
func (s *KeeperSuite) TestMsgAddToTierPosition_MultipleCalls_NoDivergence() {
	initialAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(initialAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Slash to get non-1:1 rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))
	addPerCall := sdkmath.NewInt(1001) // odd to maximize truncation
	numAdds := 5
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	posId := positions[0].Id

	_, bondDenom := s.getStakingData()
	totalNeeded := addPerCall.MulRaw(int64(numAdds))
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr, sdk.NewCoins(sdk.NewCoin(bondDenom, totalNeeded)))
	s.Require().NoError(err)
	for i := 0; i < numAdds; i++ {
		_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
			Owner:      addr.String(),
			PositionId: posId,
			Amount:     addPerCall,
		})
		s.Require().NoError(err)
	}

	pos, err = s.keeper.GetPosition(s.ctx, posId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"after %d additions, pos.Amount must equal actual token value from shares", numAdds)

	// The reconciled amount must be strictly less than the arithmetic sum.
	// Without the fix, pos.Amount would equal arithmeticSum (15005), diverging
	// from the actual share-backed value by up to numAdds tokens.
	arithmeticSum := initialAmount.Add(addPerCall.MulRaw(int64(numAdds)))
	s.Require().NotEqual(arithmeticSum.String(), pos.Amount.String(),
		"pos.Amount must differ from naive arithmetic sum due to truncation")
}

// TestMsgWithdrawFromTier_AfterUndelegate_NoInsolvency verifies the full
// lifecycle: lock → delegate → exit → undelegate → withdraw. The module
// account should have exactly enough tokens for withdrawal after the
// reconciliation fix, without needing extra manual funding.
func (s *KeeperSuite) TestMsgWithdrawFromTier_AfterUndelegate_NoInsolvency() {
	vals, bondDenom := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash to get non-1:1 rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))

	lockAmount := sdkmath.NewInt(10001)
	pos := s.setupNewTierPosition(lockAmount, true)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)

	s.advancePastExitDuration()
	// Undelegate — this reconciles pos.Amount with actual return value.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	reconciledAmount := pos.Amount

	// Advance time past exit unlock.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(366 * 24 * time.Hour))

	s.completeStakingUnbonding(valAddr)

	// Withdrawal should succeed — the module has exactly enough tokens.
	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(reconciledAmount.String(), resp.Amount.AmountOf(bondDenom).String(),
		"withdrawn amount should equal reconciled amount")
}

// TestMsgLockTier_WithValidator_ReconcilesAmount: after LockTier with a
// validator at non-1:1 exchange rate, pos.Amount matches the actual
// share-backed token value, not the original msg.Amount.
func (s *KeeperSuite) TestMsgLockTier_WithValidator_ReconcilesAmount() {
	lockAmount := sdkmath.NewInt(10001)
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	// Slash to create non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	// pos.Amount must equal what the validator says the shares are worth.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from shares after LockTier")

	// With non-1:1 rate, reconciled amount should differ from msg.Amount.
	s.Require().NotEqual(lockAmount.String(), pos.Amount.String(),
		"pos.Amount must differ from msg.Amount due to truncation")
}

// TestMsgCommitDelegationToTier_ReconcilesAmount: after CommitDelegationToTier
// at non-1:1 exchange rate, pos.Amount matches the actual share-backed token
// value, not the original msg.Amount.
func (s *KeeperSuite) TestMsgCommitDelegationToTier_ReconcilesAmount() {
	s.setupTier(1)
	delAddr, valAddr := s.getDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash to create non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%

	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	commitAmount := val.TokensFromShares(del.Shares).TruncateInt().Quo(sdkmath.NewInt(2))

	_, err = msgServer.CommitDelegationToTier(s.ctx, &types.MsgCommitDelegationToTier{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Id:               1,
		Amount:           commitAmount,
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos := positions[0]

	// Re-fetch validator for current exchange rate.
	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from shares after CommitDelegationToTier")
}

// TestMsgTierDelegate_ReconcilesAmount: after LockTier at non-1:1
// exchange rate, pos.Amount matches the actual share-backed token value.
func (s *KeeperSuite) TestMsgTierDelegate_ReconcilesAmount() {
	lockAmount := sdkmath.NewInt(10001)
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Slash to create non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	posId := positions[0].Id

	pos, err = s.keeper.GetPosition(s.ctx, posId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from shares after LockTier")

	// With non-1:1 rate, reconciled amount should differ from original lockAmount.
	s.Require().NotEqual(lockAmount.String(), pos.Amount.String(),
		"pos.Amount must differ from original lockAmount due to truncation")
}

// TestMsgTierRedelegate_ReconcilesAmount: after TierRedelegate at non-1:1
// exchange rate, pos.Amount matches the destination validator's share-backed
// token value.
func (s *KeeperSuite) TestMsgTierRedelegate_ReconcilesAmount() {
	lockAmount := sdkmath.NewInt(10001)
	_, bondDenom := s.getStakingData()
	pos := s.setupNewTierPosition(lockAmount, false)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	dstValAddr, _ := s.createSecondValidator()

	// Slash source validator to create non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	posId := positions[0].Id

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)

	// Redelegate to destination validator — this should reconcile pos.Amount.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        addr.String(),
		PositionId:   posId,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, posId)
	s.Require().NoError(err)

	// Verify pos.Amount matches destination validator's share value.
	dstVal, err := s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	actualTokenValue := dstVal.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from destination validator's shares after TierRedelegate")
}

// TestWithdrawFromTier_FailsWithPendingUnbonding verifies that withdrawal is
// blocked when unbonding entries are still pending (mapping exists).
func (s *KeeperSuite) TestWithdrawFromTier_FailsWithPendingUnbonding() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(100000), bondDenom)

	s.advancePastExitDuration()
	// Undelegate — this creates an unbonding mapping.
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Verify unbonding mapping exists.
	hasUnbonding, err := s.keeper.StillUnbonding(s.ctx, 0)
	s.Require().NoError(err)
	s.Require().True(hasUnbonding, "unbonding mapping should exist after TierUndelegate")

	// Advance time past exit lock duration.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(newTestTier(1).ExitDuration * 2))

	// Withdrawal should fail because unbonding entries are pending.
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionUnbonding)

	// Simulate unbonding completion via hook.
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	hooks := s.keeper.Hooks()
	// Get the unbonding IDs for position 0.
	iter, err := s.keeper.UnbondingDelegationMappings.Indexes.ByPosition.MatchExact(s.ctx, uint64(0))
	s.Require().NoError(err)
	unbondingIds, err := iter.PrimaryKeys()
	s.Require().NoError(err)
	s.Require().NotEmpty(unbondingIds)

	err = hooks.AfterUnbondingCompleted(s.ctx, poolAddr, valAddr, unbondingIds)
	s.Require().NoError(err)

	// Verify mapping is cleaned up.
	hasUnbonding, err = s.keeper.StillUnbonding(s.ctx, 0)
	s.Require().NoError(err)
	s.Require().False(hasUnbonding, "unbonding mapping should be cleaned up after hook")

	s.completeStakingUnbonding(valAddr)

	// Withdrawal should now pass since unbonding entries are cleaned up.
	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
}
