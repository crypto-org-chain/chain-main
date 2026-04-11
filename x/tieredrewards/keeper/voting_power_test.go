package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestGetVotingPowerForAddress_NoDelegatedPositions() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	s.advancePastExitDuration()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: pos.Id})
	s.Require().NoError(err)

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(), "undelegated position should contribute zero voting power")
}

func (s *KeeperSuite) TestGetVotingPowerForAddress_DelegatedPosition() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"delegated position should contribute its full amount as voting power")
}

func (s *KeeperSuite) TestGetVotingPowerForAddress_MultiplePositions() {
	// Position 1: delegated, 3000
	pos := s.setupNewTierPosition(sdkmath.NewInt(3000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	tier2 := newTestTier(2)
	tier2.MinLockAmount = sdkmath.NewInt(100)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier2))

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Position 2: delegated, 2000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
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
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Before triggering exit: position is active, should have full voting power.
	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
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
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Exit has been triggered immediately: position is still delegated — per
	// ADR-006 §8.5 it should still contribute voting power.
	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exit triggered but still delegated: should have voting power; got %s", power)

	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	power, err = s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().True(power.IsZero(),
		"after undelegate, position should no longer contribute voting power")
}

// TestGetVotingPowerForAddress_UnbondingValidatorNotCounted verifies that
// delegated positions on an unbonding validator contribute zero governance
// power, consistent with standard gov tally semantics.
func (s *KeeperSuite) TestGetVotingPowerForAddress_UnbondingValidatorNotCounted() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	s.jailAndUnbondValidator(valAddr)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "validator should no longer be bonded")

	positions, err := s.keeper.GetDelegatedPositionsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	power, err := s.keeper.GetVotingPowerForAddress(s.ctx, delAddr)
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
	pos := s.setupNewTierPosition(sdkmath.NewInt(3000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create a second position, delegate with immediate exit, then undelegate: 2000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(2000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

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
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

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

// TestZeroAmountPositiveSharesState verifies that after a full (100%) staking
// slash, the validator's tokens go to zero and voting power is naturally zero
// via TokensFromShares, even though the position still has positive
// DelegatedShares.
func (s *KeeperSuite) TestZeroAmountPositiveSharesState() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, false)
	s.Require().True(pos.DelegatedShares.IsPositive(), "test setup failed: expected positive delegated shares")

	// Slash the validator through staking with 100% fraction so tokens are
	// actually burned and the exchange rate drops to zero.
	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	power := val.GetConsensusPower(s.app.StakingKeeper.PowerReduction(s.ctx))
	_, err = s.app.StakingKeeper.Slash(s.ctx, consAddr, s.ctx.BlockHeight(), power, sdkmath.LegacyOneDec())
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.IsZero(), "expected position amount to be zero after slash")
	s.Require().True(posAfter.IsDelegated(), "expected position to remain delegated")
	s.Require().True(posAfter.DelegatedShares.IsPositive(), "expected delegated shares to remain positive")

	voter, err := sdk.AccAddressFromBech32(pos.Owner)
	s.Require().NoError(err)
	votingPower, err := s.keeper.GetVotingPowerForAddress(s.ctx, voter)
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
// triggered exit are still included in the total per ADR-006 §8.5.
func (s *KeeperSuite) TestTotalDelegatedVotingPower_IncludesExiting() {
	// Active delegated position: 3000
	pos := s.setupNewTierPosition(sdkmath.NewInt(3000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Delegated but immediately exiting: 2000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
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
