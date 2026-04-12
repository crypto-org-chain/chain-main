package keeper_test

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------------------------------------------------------------------------
// slashRedelegationPosition tests (AfterRedelegationSlashed)
// ---------------------------------------------------------------------------

// Redelegation slash reduces both Amount and DelegatedShares.
func (s *KeeperSuite) TestSlashRedelegationPosition_ReducesBoth() {
	lockAmount := sdkmath.NewInt(10000)

	pos, unbondingId := s.setupRedelegatingPosition(lockAmount)
	origShares := pos.DelegatedShares
	s.Require().True(origShares.IsPositive())

	// Use 10% of actual shares so the position stays delegated after slash.
	slashTokens := sdkmath.NewInt(1000)
	shareBurnt := origShares.Quo(sdkmath.LegacyNewDec(10))

	err := s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.Amount.Equal(lockAmount.Sub(slashTokens)),
		"Amount should be reduced; got %s, want %s", updated.Amount, lockAmount.Sub(slashTokens))
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
	s.Require().True(updated.Amount.IsZero(),
		"Amount should be zero when all shares are destroyed")
}

// Unknown unbondingId is a no-op (non-tier delegation).
func (s *KeeperSuite) TestSlashRedelegationPosition_UnknownId() {
	s.setupTier(1)

	err := s.keeper.Hooks().AfterRedelegationSlashed(
		s.ctx, 999, sdkmath.NewInt(100), sdkmath.LegacyNewDec(50))
	s.Require().NoError(err) // no-op, no error
}

// Unbonding delegation slash reduces Amount but keeps DelegatedShares unchanged.
func (s *KeeperSuite) TestSlashUnbondingDelegationPosition_ReducesAmountOnly() {
	lockAmount := sdkmath.NewInt(6000)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	pos, unbondingId := s.setupUnbondingPosition(lockAmount)
	origShares := pos.DelegatedShares
	slashTokens := sdkmath.NewInt(900)

	err = s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, unbondingId, slashTokens)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.Amount.Equal(lockAmount.Sub(slashTokens)))
	s.Require().True(updated.DelegatedShares.Equal(origShares),
		"DelegatedShares should not change for unbonding slash callbacks")
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
	s.Require().True(updated.Amount.IsZero(), "Amount should floor at zero when slash exceeds position amount")
}

// Unknown unbonding IDs should be no-op for both unbonding slash callbacks.
func (s *KeeperSuite) TestSlashUnbondingPosition_UnknownIdNoOp() {
	s.setupTier(1)

	err := s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, 999, sdkmath.NewInt(100))
	s.Require().NoError(err)

	err = s.keeper.Hooks().AfterUnbondingRedelegationSlashed(s.ctx, 1000, sdkmath.NewInt(200))
	s.Require().NoError(err)
}

// ---------------------------------------------------------------------------
// Bonded slash (BeforeValidatorSlashed) regression — DelegatedShares must NOT change.
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestBondedSlash_DelegatedSharesUnchanged() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	origShares := pos.DelegatedShares

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// 1/3 slash yields a fractional amount so this test also pins truncation.
	fraction := sdkmath.LegacyMustNewDecFromStr("0.333333333333333333")
	expectedDec := valBefore.TokensFromShares(origShares).Mul(sdkmath.LegacyOneDec().Sub(fraction))
	expectedAmount := expectedDec.TruncateInt()
	s.Require().False(expectedDec.IsInteger(), "test assumption: expected post-slash amount should include fractional dust")

	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, fraction)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.Amount.Equal(expectedAmount),
		"Amount should equal truncated post-slash token value; got %s want %s",
		updated.Amount, expectedAmount)
	s.Require().True(updated.Amount.LT(pos.Amount), "Amount should be reduced after bonded slash")
	s.Require().True(updated.DelegatedShares.Equal(origShares),
		"DelegatedShares must NOT change on bonded slash; got %s, want %s",
		updated.DelegatedShares, origShares)
	s.Require().True(updated.IsDelegated())
}
