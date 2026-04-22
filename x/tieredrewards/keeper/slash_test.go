package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"
)

// setupRedelegatingPosition creates a position with redelegation
func (s *KeeperSuite) setupRedelegatingPosition(lockAmount sdkmath.Int) (types.Position, uint64) {
	s.T().Helper()
	pos := s.setupNewTierPosition(lockAmount, false)

	dstValAddr, _ := s.createSecondValidator()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	res, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	updatedPos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	return updatedPos, res.UnbondingId
}

// setupUnbondingPosition creates a position and maps it to the given
// unbonding ID via UnbondingDelegationMappings, simulating a position
// whose unbonding delegation can be slashed.
func (s *KeeperSuite) setupUnbondingPosition(lockAmount sdkmath.Int) (types.Position, uint64) {
	s.T().Helper()
	pos := s.setupNewTierPosition(lockAmount, true)
	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	res, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	updatedPos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	return updatedPos, res.UnbondingId
}

// ---------------------------------------------------------------------------
// slashRedelegationPosition tests (AfterRedelegationSlashed)
// ---------------------------------------------------------------------------

// Redelegation slash reduces DelegatedShares. UndelegatedAmount stays at 0
// for delegated positions.
func (s *KeeperSuite) TestSlashRedelegationPosition_ReducesDelegatedShares() {
	lockAmount := sdkmath.NewInt(10000)

	pos, unbondingId := s.setupRedelegatingPosition(lockAmount)
	origShares := pos.DelegatedShares
	s.Require().True(origShares.IsPositive())
	s.Require().True(pos.UndelegatedAmount.IsZero(), "delegated position should have UndelegatedAmount = 0")

	// Use 10% of actual shares so the position stays delegated after slash.
	slashTokens := sdkmath.NewInt(1000)
	shareBurnt := origShares.Quo(sdkmath.LegacyNewDec(10))

	err := s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.UndelegatedAmount.IsZero(),
		"UndelegatedAmount should remain zero for delegated position after redelegation slash")
	s.Require().True(updated.DelegatedShares.Equal(origShares.Sub(shareBurnt)),
		"DelegatedShares should be reduced; got %s, want %s", updated.DelegatedShares, origShares.Sub(shareBurnt))
	s.Require().True(updated.IsDelegated(), "position should still be delegated")
}

// When all shares are burnt, the position should clear its delegation and set amount to zero.
func (s *KeeperSuite) TestSlashRedelegationPosition_AllSharesBurnt() {
	lockAmount := sdkmath.NewInt(5000)

	pos, unbondingId := s.setupRedelegatingPosition(lockAmount)
	shares := pos.DelegatedShares

	// all tokens should be slashed if all shares are burnt
	slashTokens := lockAmount
	shareBurnt := shares.Add(sdkmath.LegacyOneDec())

	err := s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().False(updated.IsDelegated(),
		"position should have cleared delegation when shareBurnt exceeds shares")
	s.Require().True(updated.DelegatedShares.IsZero())
	s.Require().True(updated.UndelegatedAmount.IsZero(),
		"Amount should be zero when all shares are destroyed")
}

// Unknown unbondingId is a no-op (non-tier delegation).
func (s *KeeperSuite) TestSlashRedelegationPosition_UnknownId() {
	s.setupTier(1)

	err := s.keeper.Hooks().AfterRedelegationSlashed(
		s.ctx, 999, sdkmath.NewInt(100), sdkmath.LegacyNewDec(50))
	s.Require().NoError(err) // no-op, no error
}

// ---------------------------------------------------------------------------
// slashUnbondingDelegationPosition tests (AfterUnbondingDelegationSlashed)
// ---------------------------------------------------------------------------

// Unbonding delegation slash reduces Amount.
func (s *KeeperSuite) TestSlashUnbondingDelegationPosition_ReducesAmount() {
	lockAmount := sdkmath.NewInt(6000)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	pos, unbondingId := s.setupUnbondingPosition(lockAmount)
	slashTokens := sdkmath.NewInt(900)

	err = s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, unbondingId, slashTokens)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.UndelegatedAmount.Equal(lockAmount.Sub(slashTokens)))
}

// Unbonding redelegation slash floors Amount at zero when slash exceeds Amount.
func (s *KeeperSuite) TestSlashUnbondingRedelegationPosition_FloorsAtZero() {
	lockAmount := sdkmath.NewInt(4000)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	pos, unbondingId := s.setupUnbondingPosition(lockAmount)

	err = s.keeper.Hooks().AfterUnbondingRedelegationSlashed(s.ctx, unbondingId, sdkmath.NewInt(999999))
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.UndelegatedAmount.IsZero(), "Amount should floor at zero when slash exceeds position amount")
}

// Unknown unbonding IDs should be no-op for both unbonding slash callbacks.
func (s *KeeperSuite) TestSlashUnbondingPosition_UnknownIdNoOp() {
	s.setupTier(1)

	err := s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, 999, sdkmath.NewInt(100))
	s.Require().NoError(err)

	err = s.keeper.Hooks().AfterUnbondingRedelegationSlashed(s.ctx, 1000, sdkmath.NewInt(200))
	s.Require().NoError(err)
}
