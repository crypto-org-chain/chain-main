package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestGetVotingPowerByOwner_NoDelegatedPositions() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()

	s.advancePastExitDuration()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: pos.Id})
	s.Require().NoError(err)

	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(), "undelegated position should contribute zero voting power")
}

func (s *KeeperSuite) TestGetVotingPowerByOwner_DelegatedPosition() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"delegated position should contribute its full amount as voting power")
}

func (s *KeeperSuite) TestGetVotingPowerByOwner_MultiplePositions() {
	// Position 1: delegated, 3000
	lockAmt1 := sdkmath.NewInt(3000)
	pos := s.setupNewTierPosition(lockAmt1, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	tier2 := newTestTier(2)
	tier2.MinLockAmount = sdkmath.NewInt(100)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier2))

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Fund delAddr for the second and third LockTier calls.
	lockAmt2 := sdkmath.NewInt(2000)
	lockAmt3 := sdkmath.NewInt(1000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2.Add(lockAmt3))))
	s.Require().NoError(err)

	// Position 2: delegated, 2000
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               2,
		Amount:           lockAmt2,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Position 3: delegated with immediate exit, 1000 — then undelegate to make it undelegated
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmt3,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 2})
	s.Require().NoError(err)

	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)

	expected := sdkmath.LegacyNewDec(5000) // 3000 + 2000, not 1000
	s.Require().True(power.Equal(expected),
		"voting power should be sum of delegated positions only; got %s, expected %s", power, expected)
}

func (s *KeeperSuite) TestGetVotingPowerByOwner_NoPositions() {
	addr := sdk.AccAddress([]byte("no_positions________"))

	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero())
}

// TestGetVotingPowerByOwner_ExitingPositionStillCounts verifies that a
// position which has triggered exit (but is still delegated) still contributes
// voting power.
func (s *KeeperSuite) TestGetVotingPowerByOwner_ExitingPositionStillCounts() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Before triggering exit: position is active, should have full voting power.
	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"active delegated position should contribute voting power; got %s", power)

	// Trigger exit on the position.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// After triggering exit: position is still delegated
	// it should still contribute voting power.
	power, err = s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exiting but still delegated position should contribute voting power; got %s", power)
}

// TestGetVotingPowerByOwner_TriggerExitToUndelegate tests the full lifecycle:
// exit triggered / exit duration elapsed → still has voting power
// upon undelegate → no voting power.
func (s *KeeperSuite) TestGetVotingPowerByOwner_TriggerExitToUndelegate() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()

	// Exit has been triggered immediately: position is still delegated
	// It should still contribute to voting power.
	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exit triggered but still delegated: should contribute to voting power; got %s", power)

	s.advancePastExitDuration()

	power, err = s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"after exit lock duration, position should still contribute to voting power; got %s", power)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	power, err = s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(),
		"after undelegate, position should no longer contribute voting power")
}

// TestGetVotingPowerByOwner_UnbondingValidatorNotCounted verifies that
// delegated positions on an unbonding validator contribute zero governance
// power, consistent with standard gov tally semantics.
func (s *KeeperSuite) TestGetVotingPowerByOwner_UnbondingValidatorNotCounted() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	s.jailAndUnbondValidator(valAddr)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "validator should no longer be bonded")

	positions, err := s.keeper.GetPositionStatesByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	power, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(),
		"delegated position on unbonding validator should not count; got %s", power)

	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.IsZero(),
		"total delegated voting power should be zero when validator unbonding; got %s", total)
}

func (s *KeeperSuite) TestTotalDelegatedVotingPower() {
	// Delegated: 3000
	lockAmt1 := sdkmath.NewInt(3000)
	pos := s.setupNewTierPosition(lockAmt1, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Fund delAddr for the second LockTier call.
	lockAmt2 := sdkmath.NewInt(2000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2)))
	s.Require().NoError(err)

	// Create a second position, delegate with immediate exit, then undelegate: 2000
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(2000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 1})
	s.Require().NoError(err)

	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.Equal(sdkmath.LegacyNewDec(3000)),
		"total delegated voting power should be 3000; got %s", total)
}

// TestVotingPower_AfterSlash verifies that both getVotingPowerByOwner and
// totalDelegatedVotingPower use share-based token value after
// a slash, and that they agree with each other.
func (s *KeeperSuite) TestVotingPower_AfterSlash() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	perAddrPower, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	totalPower, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(perAddrPower.Equal(totalPower),
		"before slash: voting power per-address (%s) and total (%s) should match", perAddrPower, totalPower)

	// Slash the validator through staking so tokens are actually burned and
	// the exchange rate changes.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	power := val.GetConsensusPower(s.app.StakingKeeper.PowerReduction(s.ctx))
	slashFraction := sdkmath.LegacyNewDecWithPrec(10, 2) // 10%
	_, err = s.app.StakingKeeper.Slash(s.ctx, consAddr, s.ctx.BlockHeight(), power, slashFraction)
	s.Require().NoError(err)

	// After slash: voting power should be less than lockAmount.
	perAddrPowerAfter, err := s.keeper.GetVotingPowerByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(perAddrPowerAfter.LT(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"per-address voting power should decrease after slash; got %s", perAddrPowerAfter)

	totalPowerAfter, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(totalPowerAfter.LT(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"total voting power should decrease after slash; got %s", totalPowerAfter)

	s.Require().True(perAddrPowerAfter.Equal(totalPowerAfter),
		"after slash: per-address (%s) and total (%s) must match", perAddrPowerAfter, totalPowerAfter)
}

// TestZeroAmountPositiveSharesState verifies that after a full (100%) staking
// slash, the validator's tokens go to zero and voting power is naturally zero
// via TokensFromShares, even though the position still has positive
// DelegatedShares.
func (s *KeeperSuite) TestZeroAmountPositiveSharesState() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, false)
	s.Require().True(pos.Delegation.Shares.IsPositive(), "test setup failed: expected positive delegated shares")

	// Slash the validator through staking with 100% fraction so tokens are
	// actually burned and the exchange rate drops to zero.
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	power := val.GetConsensusPower(s.app.StakingKeeper.PowerReduction(s.ctx))
	_, err = s.app.StakingKeeper.Slash(s.ctx, consAddr, s.ctx.BlockHeight(), power, sdkmath.LegacyOneDec())
	s.Require().NoError(err)

	// Apply validator set updates so the slashed validator is removed from the bonded set.
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.IsDelegated(), "expected position to remain delegated")
	s.Require().True(s.getPositionAmount(posAfter).IsZero(), "delegated position should still have zero amount after slash")
	s.Require().True(posAfter.Delegation.Shares.IsPositive(), "expected delegated shares to remain positive")

	voter := sdk.MustAccAddressFromBech32(pos.Owner)
	votingPower, err := s.keeper.GetVotingPowerByOwner(s.ctx, voter)
	s.Require().NoError(err)
	s.Require().True(votingPower.IsZero(), "zero-amount delegated position should not contribute voting power")

	totalPower, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(totalPower.IsZero(), "zero-amount delegated positions should not contribute to total delegated voting power")
}

func (s *KeeperSuite) TestTotalDelegatedVotingPower_Empty() {
	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.IsZero())
}

// TestTotalDelegatedVotingPower_IncludesExiting verifies that positions with a
// triggered exit are still included in the total.
func (s *KeeperSuite) TestTotalDelegatedVotingPower_IncludesExiting() {
	// Active delegated position: 3000
	lockAmt1 := sdkmath.NewInt(3000)
	pos := s.setupNewTierPosition(lockAmt1, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Fund delAddr for the second LockTier call.
	lockAmt2 := sdkmath.NewInt(2000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2)))
	s.Require().NoError(err)

	// Delegated but immediately exiting: 2000
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(2000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.Equal(sdkmath.LegacyNewDec(5000)),
		"exiting position should be included in total; got %s", total)
}
