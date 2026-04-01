package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestGetVotingPowerForAddress_NoDelegatedPositions() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock WITH delegation and immediate exit, then undelegate to get an undelegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(5000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(), "undelegated position should contribute zero voting power")
}

func (s *KeeperSuite) TestGetVotingPowerForAddress_DelegatedPosition() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"delegated position should contribute its full amount as voting power")
}

func (s *KeeperSuite) TestGetVotingPowerForAddress_MultiplePositions() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()

	tier2 := newTestTier(2)
	tier2.MinLockAmount = sdkmath.NewInt(100)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier2))

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Position 1: delegated, 3000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(3000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Position 2: delegated, 2000
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               2,
		Amount:           sdkmath.NewInt(2000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Position 3: delegated with immediate exit, 1000 — then undelegate to make it undelegated
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(1000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 2})
	s.Require().NoError(err)

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)

	expected := sdkmath.LegacyNewDec(5000) // 3000 + 2000, not 1000
	s.Require().True(power.Equal(expected),
		"voting power should be sum of delegated positions only; got %s, expected %s", power, expected)
}

func (s *KeeperSuite) TestGetVotingPowerForAddress_NoPositions() {
	addr := sdk.AccAddress([]byte("no_positions________"))

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero())
}

// TestGetVotingPowerForAddress_ExitingPositionStillCounts verifies that a
// position which has triggered exit (but is still delegated) still contributes
// voting power per ADR-006 §8.5.
func (s *KeeperSuite) TestGetVotingPowerForAddress_ExitingPositionStillCounts() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Before triggering exit: position is active, should have full voting power.
	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"active delegated position should contribute voting power; got %s", power)

	// Trigger exit on the position (positionId 0 is the first one).
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	// After triggering exit: position is still delegated — per ADR-006 §8.5
	// it should still contribute voting power.
	power, err = s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exiting but still delegated position should contribute voting power; got %s", power)
}

// TestGetVotingPowerForAddress_AfterUndelegate tests the full lifecycle:
// exit triggered → still has voting power; TierUndelegate → no voting power.
func (s *KeeperSuite) TestGetVotingPowerForAddress_AfterUndelegate() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Exit has been triggered immediately: position is still delegated — per
	// ADR-006 §8.5 it should still contribute voting power.
	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exit triggered but still delegated: should have voting power; got %s", power)

	// Fund rewards pool and advance past exit duration so undelegation is allowed.
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)
	s.advancePastExitDuration()

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)

	power, err = s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(),
		"after undelegate, position should no longer contribute voting power")
}

func (s *KeeperSuite) TestTotalDelegatedVotingPower() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Delegated: 3000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(3000),
		ValidatorAddress: valAddr.String(),
	})
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

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 1})
	s.Require().NoError(err)

	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.Equal(sdkmath.LegacyNewDec(3000)),
		"total delegated voting power should be 3000; got %s", total)
}

// TestVotingPower_AfterSlash verifies that both getVotingPowerForAddress and
// totalDelegatedVotingPower use share-based token value (not pos.Amount) after
// a slash, and that they agree with each other.
func (s *KeeperSuite) TestVotingPower_AfterSlash() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	perAddrPower, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
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
	perAddrPowerAfter, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
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

func (s *KeeperSuite) TestTotalDelegatedVotingPower_Empty() {
	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.IsZero())
}

// TestTotalDelegatedVotingPower_IncludesExiting verifies that positions with a
// triggered exit are still included in the total per ADR-006 §8.5.
func (s *KeeperSuite) TestTotalDelegatedVotingPower_IncludesExiting() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Active delegated position: 3000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(3000),
		ValidatorAddress: valAddr.String(),
	})
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
