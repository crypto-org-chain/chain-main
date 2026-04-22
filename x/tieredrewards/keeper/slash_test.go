package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

// ---------------------------------------------------------------------------
// slashUnbondingDelegationPosition tests (AfterUnbondingDelegationSlashed)
// ---------------------------------------------------------------------------

// Unbonding delegation slash reduces Amount.
func (s *KeeperSuite) TestSlashUnbondingDelegationPosition_ReducesAmountOnly() {
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

	s.Require().True(updated.Amount.Equal(lockAmount.Sub(slashTokens)))
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
// Bonded slash (BeforeValidatorSlashed)
// ---------------------------------------------------------------------------

// TestBondedSlash_DelegatedSharesUnchanged verifies that DelegatedShares remains unchanged after bonded slash.
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

// ---------------------------------------------------------------------------
// slashRedelegationPosition — reward claiming before slash
// ---------------------------------------------------------------------------

// TestSlashRedelegationPosition_ClaimsRewardsBeforeSlash verifies that both
// base and bonus rewards accrued at the pre-slash share count are paid out
// before shares are reduced, and that checkpoints are advanced.
func (s *KeeperSuite) TestSlashRedelegationPosition_ClaimsRewardsBeforeSlash() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos, unbondingId := s.setupRedelegatingPosition(lockAmount)

	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Set zero commission so delegators receive base rewards.
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Fund the bonus pool and allocate base rewards.
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(500_000), bondDenom)

	// Advance block height so updateBaseRewardsPerShare will collect new rewards
	// (it skips re-collection within the same block via transient store guard).
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Advance time so bonus accrues.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	s.Require().NoError(err)
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, bondDenom)

	// Slash via redelegation — this should claim all rewards first.
	slashTokens := sdkmath.NewInt(1000)
	shareBurnt := pos.DelegatedShares.Quo(sdkmath.LegacyNewDec(10))
	err = s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Base: BaseRewardsPerShare must have advanced.
	s.Require().True(len(posAfter.BaseRewardsPerShare) > 0,
		"BaseRewardsPerShare should be set after slash claims rewards")
	s.Require().True(
		!posAfter.BaseRewardsPerShare.Equal(posBefore.BaseRewardsPerShare),
		"BaseRewardsPerShare should advance; before=%s, after=%s",
		posBefore.BaseRewardsPerShare, posAfter.BaseRewardsPerShare)

	// Checkpoint: LastBonusAccrual must have advanced.
	s.Require().True(posAfter.LastBonusAccrual.After(posBefore.LastBonusAccrual),
		"LastBonusAccrual should advance; before=%s, after=%s",
		posBefore.LastBonusAccrual, posAfter.LastBonusAccrual)

	// owner balance must have increased.
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount),
		"owner should have received rewards during redelegation slash; before=%s, after=%s",
		balBefore.Amount, balAfter.Amount)
}

// TestSlashRedelegationPosition_InsufficientBonusPool verifies that
// the slash proceeds even when the bonus pool cannot cover accrued bonus.
func (s *KeeperSuite) TestSlashRedelegationPosition_InsufficientBonusPool() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos, unbondingId := s.setupRedelegatingPosition(lockAmount)

	// Do NOT fund the bonus pool — it's empty.
	// Advance time so bonus would be positive if pool had funds.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Slash should succeed despite empty bonus pool.
	slashTokens := sdkmath.NewInt(1000)
	shareBurnt := pos.DelegatedShares.Quo(sdkmath.LegacyNewDec(10))
	err := s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err, "slash should not fail when bonus pool is empty")

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.IsDelegated(), "position should still be delegated")
	s.Require().True(updated.DelegatedShares.LT(pos.DelegatedShares),
		"shares should be reduced despite insufficient bonus pool")
}
