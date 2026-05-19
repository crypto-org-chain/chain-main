package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// setupRedelegatingPosition creates a position, redelegates it to a second
// validator, and returns the position state, the destination validator address,
// and the staking unbonding id assigned to the redelegation entry.
func (s *KeeperSuite) setupRedelegatingPosition(lockAmount sdkmath.Int) (types.PositionState, sdk.ValAddress, uint64) {
	s.T().Helper()
	pos := s.setupNewTierPosition(lockAmount, false)

	dstValAddr, _ := s.createSecondValidator()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	updatedPos, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Walk RedelegationMappings to find the unbonding id for this position.
	var unbondingID uint64
	err = s.keeper.RedelegationMappings.Walk(s.ctx, nil, func(id, posId uint64) (bool, error) {
		if posId == pos.Id {
			unbondingID = id
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().NotZero(unbondingID, "expected redelegation mapping for position after TierRedelegate")

	return updatedPos, dstValAddr, unbondingID
}

// ---------------------------------------------------------------------------
// slashRedelegationPosition tests (BeforeRedelegationSlashed)
// ---------------------------------------------------------------------------

// Unknown unbondingId is a no-op (non-tier redelegation).
func (s *KeeperSuite) TestSlashRedelegationPosition_UnknownUnbondingID() {
	s.setupTier(1)

	err := s.keeper.Hooks().BeforeRedelegationSlashed(
		s.ctx, 99999, sdkmath.LegacyNewDec(50))
	s.Require().NoError(err) // no-op, no error
}

// TestSlashRedelegationPosition_ClaimsBonusRewardsUpToSlash verifies that when
// a redelegation slash fires, any bonus accrued on the destination delegation
// since the last accrual checkpoint is paid out to the position owner, and the
// position's bonus-state checkpoints (LastBonusAccrual, LastKnownBonded) are
// advanced.
func (s *KeeperSuite) TestSlashRedelegationPosition_ClaimsBonusRewardsUpToSlash() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	pos, _, unbondingID := s.setupRedelegatingPosition(lockAmount)
	owner := sdk.MustAccAddressFromBech32(pos.Owner)
	preAccrual := pos.LastBonusAccrual

	// Advance block time so bonus accrues on the destination validator.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)

	// Partial slash — a small fraction of shares.
	sharesToUnbond := pos.Delegation.Shares.Quo(sdkmath.LegacyNewDec(10))
	err := s.keeper.Hooks().BeforeRedelegationSlashed(s.ctx, unbondingID, sharesToUnbond)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount),
		"owner should have received bonus rewards accrued up to slash: before=%s after=%s",
		balBefore.Amount, balAfter.Amount)

	updated, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.LastBonusAccrual.After(preAccrual),
		"LastBonusAccrual should have advanced past the pre-slash checkpoint")
	s.Require().Equal(s.ctx.BlockTime(), updated.LastBonusAccrual,
		"LastBonusAccrual should advance to the slash block time")
	s.Require().True(updated.LastKnownBonded,
		"LastKnownBonded should remain true — destination validator is still bonded")
}

// TestSlashRedelegationPosition_PartialSlashPaysBonusOnPreSlashShares ensures
// that bonus is calculated and claimed on pre-slash shares. Compares the
// owner's balance delta against ComputeSegmentBonus run on the pre-slash
// PositionState so the test stays in sync if the formula changes.
func (s *KeeperSuite) TestSlashRedelegationPosition_PartialSlashPaysBonusOnPreSlashShares() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	pos, dstValAddr, unbondingID := s.setupRedelegatingPosition(lockAmount)
	owner := sdk.MustAccAddressFromBech32(pos.Owner)
	segmentStart := pos.LastBonusAccrual

	// Advance time so bonus accrues on the destination validator.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Expected bonus via the keeper's own segment-bonus formula on the
	// PRE-slash PositionState.
	tier, err := s.keeper.GetTier(s.ctx, pos.TierId)
	s.Require().NoError(err)
	dstVal, err := s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	tokensPerShare := dstVal.TokensFromShares(sdkmath.LegacyOneDec())
	expectedBonus := s.keeper.ComputeSegmentBonus(pos, tier, segmentStart, s.ctx.BlockTime(), tokensPerShare)
	s.Require().True(expectedBonus.IsPositive(),
		"test fixture error: expected bonus should be positive (got %s)", expectedBonus)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)

	// Partial slash — hook fires with 50% of shares.
	sharesToUnbond := pos.Delegation.Shares.Quo(sdkmath.LegacyNewDec(2))
	err = s.keeper.Hooks().BeforeRedelegationSlashed(s.ctx, unbondingID, sharesToUnbond)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)
	actualBonus := balAfter.Amount.Sub(balBefore.Amount)
	s.Require().True(actualBonus.Equal(expectedBonus),
		"bonus must match ComputeSegmentBonus on pre-slash PositionState: expected=%s, got=%s",
		expectedBonus, actualBonus)

	// Checkpoints advanced by processEventsAndClaimBonus.
	updated, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.LastBonusAccrual.Equal(s.ctx.BlockTime()),
		"LastBonusAccrual should advance to the slash block time")
}

// TestSlashRedelegationPosition_FullSlashStillPaysBonus ensures that on a 100%
// slash the owner still receives the full pre-slash bonus. Under the old
// AfterRedelegationSlashed path this was always zero because pos.Delegation
// was nil by the time processEventsAndClaimBonus ran.
func (s *KeeperSuite) TestSlashRedelegationPosition_FullSlashStillPaysBonus() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	pos, dstValAddr, unbondingID := s.setupRedelegatingPosition(lockAmount)
	owner := sdk.MustAccAddressFromBech32(pos.Owner)
	segmentStart := pos.LastBonusAccrual

	// Advance time so bonus accrues on the destination validator.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Expected bonus via the keeper's own segment-bonus formula on the
	// PRE-slash PositionState.
	tier, err := s.keeper.GetTier(s.ctx, pos.TierId)
	s.Require().NoError(err)
	dstVal, err := s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	tokensPerShare := dstVal.TokensFromShares(sdkmath.LegacyOneDec())
	expectedBonus := s.keeper.ComputeSegmentBonus(pos, tier, segmentStart, s.ctx.BlockTime(), tokensPerShare)
	s.Require().True(expectedBonus.IsPositive(),
		"test fixture error: expected bonus should be positive (got %s)", expectedBonus)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)

	// Full slash — pass full share count.
	err = s.keeper.Hooks().BeforeRedelegationSlashed(s.ctx, unbondingID, pos.Delegation.Shares)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)
	actualBonus := balAfter.Amount.Sub(balBefore.Amount)
	s.Require().True(actualBonus.Equal(expectedBonus),
		"bonus must match ComputeSegmentBonus on pre-slash PositionState even on full slash: expected=%s, got=%s",
		expectedBonus, actualBonus)

	updated, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.LastBonusAccrual.IsZero(), "ResetBonusCheckpoints should have zeroed LastBonusAccrual")
	s.Require().False(updated.LastKnownBonded, "ResetBonusCheckpoints should have cleared LastKnownBonded")

	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, dstValAddr)
	s.Require().ErrorIs(err, collections.ErrNotFound,
		"dst validator counter must be decremented to zero (entry removed) on full slash")
}

// TestRedelegationMapping_SurvivesZombieCompletion ensures that
// with per-unbondingId granularity, maturity of a zombie (zero-balance)
// redelegation entry only removes its own mapping row and leaves any fresh
// concurrent redelegation's mapping intact.
// Zombie redeelgation entry is possible when a redelegation has not matured but its delegation has been slashed to zero.
// This means that the user can delegate its position once again (after locking more funds), and redelegate again, resulting in 2 redelegation entries in total linking to the same position id.
func (s *KeeperSuite) TestRedelegationMapping_SurvivesZombieCompletion() {
	positionId := uint64(7)
	zombieID := uint64(101)
	freshID := uint64(102)

	s.Require().NoError(s.keeper.RedelegationMappings.Set(s.ctx, zombieID, positionId))
	s.Require().NoError(s.keeper.RedelegationMappings.Set(s.ctx, freshID, positionId))

	// AfterRedelegationCompleted fires for the matured zombie only.
	delAddr := types.GetDelegatorAddress(positionId)
	src := sdk.ValAddress([]byte("old_src_val_________"))
	dst := sdk.ValAddress([]byte("old_dst_val_________"))
	err := s.keeper.Hooks().AfterRedelegationCompleted(s.ctx, delAddr, src, dst, []uint64{zombieID})
	s.Require().NoError(err)

	// Zombie row gone, fresh row survives — completion of one unbondingId MUST
	// NOT clobber unrelated rows that share a position id.
	has, err := s.keeper.RedelegationMappings.Has(s.ctx, zombieID)
	s.Require().NoError(err)
	s.Require().False(has, "zombie mapping row should be deleted by AfterRedelegationCompleted")

	has, err = s.keeper.RedelegationMappings.Has(s.ctx, freshID)
	s.Require().NoError(err)
	s.Require().True(has, "fresh mapping row must survive zombie completion")

	// When the fresh redelegation matures, its mapping row also clears.
	err = s.keeper.Hooks().AfterRedelegationCompleted(s.ctx, delAddr, src, dst, []uint64{freshID})
	s.Require().NoError(err)

	has, err = s.keeper.RedelegationMappings.Has(s.ctx, freshID)
	s.Require().NoError(err)
	s.Require().False(has, "fresh mapping row should be deleted once it also matures")
}
