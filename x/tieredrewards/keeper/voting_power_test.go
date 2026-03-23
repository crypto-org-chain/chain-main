package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestGetVotingPowerForAddress_NoDelegatedPositions() {
	delAddr, _, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock without delegating
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(5000),
	})
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
	delAddr, valAddr, _ := s.setupTierAndDelegator()

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

	// Position 3: NOT delegated, 1000
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(1000),
	})
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

// TestGetVotingPowerForAddress_ExitingPositionIgnored verifies that a position
// which has triggered exit (but is still delegated) no longer contributes
// voting power, while the same position before exit does contribute.
func (s *KeeperSuite) TestGetVotingPowerForAddress_ExitingPositionIgnored() {
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

	// After triggering exit: position is still delegated but exit is in progress —
	// it should no longer contribute voting power.
	power, err = s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(),
		"exiting position should not contribute voting power; got %s", power)
}

// TestGetVotingPowerForAddress_AfterUndelegate tests the full lifecycle:
// exit triggered → no voting power; TierUndelegate → still no voting power.
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

	// Exit has been triggered immediately: position is still delegated but the
	// exit is in progress, so it must not contribute voting power.
	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(),
		"exit triggered: should have zero voting power while exit is pending; got %s", power)

	// Fund rewards pool and advance time past exit duration
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(366 * 24 * time.Hour)) // 366 days

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
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Delegated: 3000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(3000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Not delegated: 2000
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  delAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(2000),
	})
	s.Require().NoError(err)

	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.Equal(sdkmath.LegacyNewDec(3000)),
		"total delegated voting power should be 3000; got %s", total)
}

func (s *KeeperSuite) TestTotalDelegatedVotingPower_Empty() {
	total, err := s.keeper.TotalDelegatedVotingPower(s.ctx)
	s.Require().NoError(err)
	s.Require().True(total.IsZero())
}

// TestTotalDelegatedVotingPower_ExcludesExiting verifies that positions with a
// triggered exit are excluded from the total.
func (s *KeeperSuite) TestTotalDelegatedVotingPower_ExcludesExiting() {
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
	s.Require().True(total.Equal(sdkmath.LegacyNewDec(3000)),
		"exiting position should be excluded from total; got %s", total)
}
