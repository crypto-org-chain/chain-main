package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
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
	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().Equal(freshAddr.String(), pos.Owner)
	s.Require().True(pos.Amount.IsZero(), "delegated positions have Amount=0")
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))
	s.Require().Equal(uint64(0), pos.LastEventSeq, "LastEventSeq should be 0 for fresh validator")
}

func (s *KeeperSuite) TestMsgLockTier_LastEventSeqSkipsPriorEvents() {
	// Step 1: Create a first position to establish validator count.
	lockAmt := sdkmath.NewInt(1000)
	pos1 := s.setupNewTierPosition(lockAmt, false)
	valAddr := sdk.MustValAddressFromBech32(pos1.Validator)

	// Step 2: Record a slash event via the staking hook.
	err := s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	// Step 3: Create a second position via LockTier on the same validator.
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

	pos2, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)

	// Step 4: The second position's LastEventSeq should equal 1 (the slash event seq).
	s.Require().Equal(uint64(1), pos2.LastEventSeq,
		"new position should skip prior events; LastEventSeq should be 1 (the slash event)")

	// Step 5: The first position's LastEventSeq should still be 0 (set at creation, before the slash).
	pos1, err = s.keeper.GetPosition(s.ctx, pos1.Id)
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

	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
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

func (s *KeeperSuite) TestMsgLockTier_UpdateBaseRewardsPerShare_FirstPosition() {
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

func (s *KeeperSuite) TestMsgLockTier_UpdateBaseRewardsPerShare_SecondPositionGetsUpdatedRatio() {
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
