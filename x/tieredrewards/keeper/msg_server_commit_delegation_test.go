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

func (s *KeeperSuite) TestMsgCommitDelegationToTier_UpdateBaseRewardsPerShare_FirstPosition() {
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

func (s *KeeperSuite) TestMsgCommitDelegationToTier_UpdateBaseRewardsPerShare_SecondPositionGetsUpdatedRatio() {
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
