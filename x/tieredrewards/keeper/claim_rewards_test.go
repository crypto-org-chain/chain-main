package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------------------------------------------------------------------------
// claimRewards
// ---------------------------------------------------------------------------

// TestClaimRewards_Undelegated verifies that claimRewards
// returns early with zero rewards for an undelegated position.
func (s *KeeperSuite) TestClaimRewards_Undelegated() {
	posSetup := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(posSetup.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, bondDenom := s.getStakingData()

	s.advancePastExitDuration()

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: posSetup.Id,
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPositionState(s.ctx, posSetup.Id)
	s.Require().NoError(err)

	_, base, bonus, err := s.keeper.ClaimRewards(s.ctx, pos)
	s.Require().NoError(err)
	s.Require().True(base.IsZero(), "base should be zero for undelegated position")
	s.Require().True(bonus.IsZero(), "bonus should be zero for undelegated position")
}

// TestClaimRewards_Delegated verifies that claimRewards
// returns base+bonus rewards for a delegated position.
func (s *KeeperSuite) TestClaimRewards_Delegated() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Allocate rewards, advance block and time so both base and bonus accrue.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(100_000_000), bondDenom)

	_, base, bonus, err := s.keeper.ClaimRewards(s.ctx, pos)
	s.Require().NoError(err)
	s.Require().True(base.IsAllPositive(), "base should be positive for delegated position")
	s.Require().True(bonus.IsAllPositive(), "bonus should be positive for delegated position")
}

// ---------------------------------------------------------------------------
// claimRewardsForPositions
// ---------------------------------------------------------------------------

// TestClaimRewardsForPositions_MixedDelegatedUndelegated verifies that when
// batch-claiming for a mix of delegated and undelegated positions, only the
// delegated position earns rewards and all positions are persisted.
func (s *KeeperSuite) TestClaimRewardsForPositions_MixedDelegatedUndelegated() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	// Position 0: delegated.
	pos0 := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos0.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Position 1: will be undelegated (same owner for simplicity).
	pos1 := s.setupNewTierPosition(lockAmount, true)
	addr1 := sdk.MustAccAddressFromBech32(pos1.Owner)

	// Undelegate position 1.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: addr1.String(), PositionId: pos1.Id,
	})
	s.Require().NoError(err)

	pos1After, err := s.keeper.GetPositionState(s.ctx, pos1.Id)
	s.Require().NoError(err)
	s.Require().False(pos1After.IsDelegated(), "pos1 should be undelegated")

	// Allocate rewards and advance block + time.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(100_000_000), bondDenom)

	// Batch claim for owner of pos0 (delegated).
	base0, bonus0, err := s.keeper.ClaimRewardsAndUpdatesPositions(s.ctx, []types.PositionState{pos0})
	s.Require().NoError(err)
	s.Require().True(base0.IsAllPositive(), "delegated position should earn base rewards")
	s.Require().True(bonus0.IsAllPositive(), "delegated position should earn bonus rewards")

	// Batch claim for owner of pos1 (undelegated).
	base1, bonus1, err := s.keeper.ClaimRewardsAndUpdatesPositions(s.ctx, []types.PositionState{pos1After})
	s.Require().NoError(err)
	s.Require().True(base1.IsZero(), "undelegated position should earn zero base rewards")
	s.Require().True(bonus1.IsZero(), "undelegated position should earn zero bonus rewards")

	// Verify both positions are persisted in store.
	persistedPos0, err := s.keeper.GetPositionState(s.ctx, pos0.Id)
	s.Require().NoError(err)
	s.Require().True(persistedPos0.IsDelegated(), "pos0 should still be delegated")

	persistedPos1, err := s.keeper.GetPositionState(s.ctx, pos1.Id)
	s.Require().NoError(err)
	s.Require().False(persistedPos1.IsDelegated(), "pos1 should still be undelegated")
}

// ---------------------------------------------------------------------------
// claimRewardsAndUpdatePositionsForTier
// ---------------------------------------------------------------------------

// TestClaimRewardsAndUpdatePositionsForTier_ClaimsAll verifies that the
// tier-sweep function claims rewards for all delegated positions in a tier.
func (s *KeeperSuite) TestClaimRewardsAndUpdatePositionsForTier_ClaimsAll() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos1 := s.setupNewTierPosition(lockAmount, false)
	addr1 := sdk.MustAccAddressFromBech32(pos1.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos1.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Create second position on the same tier and validator.
	pos2 := s.setupNewTierPosition(lockAmount, false)
	addr2 := sdk.MustAccAddressFromBech32(pos2.Owner)

	// Allocate rewards and advance block + time.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Snapshot balances.
	bal1Before := s.app.BankKeeper.GetBalance(s.ctx, addr1, bondDenom)
	bal2Before := s.app.BankKeeper.GetBalance(s.ctx, addr2, bondDenom)

	// Tier sweep.
	err := s.keeper.ClaimRewardsAndUpdateTierPositions(s.ctx, 1)
	s.Require().NoError(err)

	// Both owners should receive rewards.
	bal1After := s.app.BankKeeper.GetBalance(s.ctx, addr1, bondDenom)
	bal2After := s.app.BankKeeper.GetBalance(s.ctx, addr2, bondDenom)
	s.Require().True(bal1After.Amount.GT(bal1Before.Amount),
		"addr1 should receive rewards from tier sweep")
	s.Require().True(bal2After.Amount.GT(bal2Before.Amount),
		"addr2 should receive rewards from tier sweep")
}

// TestClaimRewardsAndUpdatePositionsForTier_SkipsUndelegated verifies that
// the tier-sweep skips undelegated positions.
func (s *KeeperSuite) TestClaimRewardsAndUpdatePositionsForTier_SkipsUndelegated() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	// Position 0: delegated.
	pos0 := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos0.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Position 1: will be undelegated.
	pos1 := s.setupNewTierPosition(lockAmount, true)
	addr2 := sdk.MustAccAddressFromBech32(pos1.Owner)

	pos1Positions, err := s.keeper.GetPositionStatesByOwner(s.ctx, addr2)
	s.Require().NoError(err)
	s.Require().Len(pos1Positions, 1)
	pos1Id := pos1Positions[0].Id

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: addr2.String(), PositionId: pos1Id,
	})
	s.Require().NoError(err)

	// Snapshot balance AFTER undelegate (so any unbonding completion that
	// happened during TierUndelegate is already reflected). Any further
	// change must come from the tier sweep itself.
	balBeforeSweep := s.app.BankKeeper.GetAllBalances(s.ctx, addr2)

	// Allocate rewards and run tier sweep.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	err = s.keeper.ClaimRewardsAndUpdateTierPositions(s.ctx, 1)
	s.Require().NoError(err)

	balAfterSweep := s.app.BankKeeper.GetAllBalances(s.ctx, addr2)
	s.Require().True(balAfterSweep.Equal(balBeforeSweep),
		"undelegated position should not receive any rewards from the tier sweep")
}

// After the validator re-bonds, bonus accrual should resume from the new bonded time.
func (s *KeeperSuite) TestBonusAccrual_ResumesAfterRebond() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Jail + apply -> unbonding (hook records UNBOND event).
	s.jailAndUnbondValidator(valAddr)

	// Claim to process the UNBOND event and advance LastBonusAccrual.
	_, _, err = s.keeper.ClaimRewardsAndUpdatesPositions(s.ctx, []types.PositionState{pos})
	s.Require().NoError(err)

	// Verify LastBonusAccrual was advanced to current block time.
	updated, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(s.ctx.BlockTime(), updated.LastBonusAccrual)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Unjail and apply to re-bond.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Unjail(s.ctx, consAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(val.IsBonded(), "validator should be bonded again after unjail + apply")

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	// Re-read position from store to get updated checkpoints from first claim.
	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Claim to process the BOND event and advance LastBonusAccrual.
	_, _, err = s.keeper.ClaimRewardsAndUpdatesPositions(s.ctx, []types.PositionState{pos})
	s.Require().NoError(err)

	updated, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(s.ctx.BlockTime(), updated.LastBonusAccrual)
}

// ---------------------------------------------------------------------------
// ProcessEventsAndClaimBonus tests -- singular events
// ---------------------------------------------------------------------------

// TestProcessEvents_SingleSlash_BonusContinuesWithRateChange verifies that a
// single slash event splits accrual into two segments at different rates, and
// a second claim at the same block time yields zero.
func (s *KeeperSuite) TestProcessEvents_SingleSlash_BonusContinuesWithRateChange() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 10 days + bump block height before slash.
	tSlash := t0.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tSlash).WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Slash 1% via the real SDK slash so the rate actually changes.
	slashFraction := sdkmath.LegacyNewDecWithPrec(1, 2)
	s.slashValidatorDirect(valAddr, slashFraction)

	// Read the slash event's snapshot rate from the store.
	evt1, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rateAtSlash := evt1.TokensPerShare

	// Advance 10 more days.
	tNow := tSlash.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Current live rate (post-slash).
	currentRate, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	// Manually compute expected bonus.
	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, t0, tSlash, rateAtSlash)
	seg2 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash, tNow, currentRate)
	expectedTotal := seg1.Add(seg2)
	s.Require().True(expectedTotal.IsPositive(), "expected bonus should be positive")

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must equal seg1+seg2 exactly")

	// Second claim at same time yields zero.
	bonus2, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))
	s.Require().True(bonus2.IsZero(), "second claim should yield zero")
}

// TestProcessEvents_SingleUnbond_GapPaysZero verifies that after an unbond
// event, the gap pays zero and only the pre-unbond bonded segment earns bonus.
func (s *KeeperSuite) TestProcessEvents_SingleUnbond_GapPaysZero() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 10 days, bump height, then unbond (jail).
	tUnbond := t0.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tUnbond).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.jailAndUnbondValidator(valAddr)

	// Read the unbond event's snapshot rate.
	evt1, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rateAtUnbond := evt1.TokensPerShare

	// Advance 30 more days (validator still unbonded).
	tNow := tUnbond.Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Expected: only the pre-unbond segment pays; gap and final segment are zero
	// because the validator is not bonded.
	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, t0, tUnbond, rateAtUnbond)
	expectedTotal := seg1
	s.Require().True(expectedTotal.IsPositive(), "expected bonus should be positive")

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must equal pre-unbond segment only")

	// Second claim yields zero.
	bonus2, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))
	s.Require().True(bonus2.IsZero(), "second claim should yield zero")
}

// TestProcessEvents_SingleBond_ResumesBonus verifies that after an immediate
// unbond and a later rebond, bonus accrues only for the post-rebond segment.
func (s *KeeperSuite) TestProcessEvents_SingleBond_ResumesBonus() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Immediately unbond (tiny pre-unbond segment).
	tUnbond := t0.Add(1 * time.Second)
	s.ctx = s.ctx.WithBlockTime(tUnbond).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.jailAndUnbondValidator(valAddr)

	// Read unbond event rate.
	evtUnbond, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rateAtUnbond := evtUnbond.TokensPerShare

	// Advance 30 days, then rebond.
	tBond := tUnbond.Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tBond).WithBlockHeight(s.ctx.BlockHeight() + 1)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Unjail(s.ctx, consAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	// Advance 10 more days bonded.
	tNow := tBond.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Current live rate for the final bonded segment.
	currentRate, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	// Expected: tiny pre-unbond + zero gap + post-rebond segment.
	segPreUnbond := s.keeper.ComputeSegmentBonus(pos, tier, t0, tUnbond, rateAtUnbond)
	segPostBond := s.keeper.ComputeSegmentBonus(pos, tier, tBond, tNow, currentRate)
	expectedTotal := segPreUnbond.Add(segPostBond)

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must equal pre-unbond + post-rebond segments exactly")
}

// ---------------------------------------------------------------------------
// ProcessEventsAndClaimBonus tests -- multi-event combinations
// ---------------------------------------------------------------------------

// TestProcessEvents_SlashThenUnbond_TwoSegments verifies that a slash followed
// by an unbond produces two bonded segments plus a zero gap.
func (s *KeeperSuite) TestProcessEvents_SlashThenUnbond_TwoSegments() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 10 days. Slash.
	tSlash := t0.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tSlash).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(1, 2)) // 1%

	evtSlash, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	slashRate := evtSlash.TokensPerShare

	// Advance 5 days. Unbond.
	tUnbond := tSlash.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tUnbond).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.jailAndUnbondValidator(valAddr)

	evtUnbond, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(2)))
	s.Require().NoError(err)
	unbondRate := evtUnbond.TokensPerShare

	// Advance 20 days (gap).
	tNow := tUnbond.Add(20 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Expected: seg1[T0, T_slash, slashRate] + seg2[T_slash, T_unbond, unbondRate] + zero gap
	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, t0, tSlash, slashRate)
	seg2 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash, tUnbond, unbondRate)
	expectedTotal := seg1.Add(seg2)

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must equal two bonded segments exactly (gap pays zero)")
}

// TestProcessEvents_UnbondBondUnbond_ThreeTransitions verifies three bond
// state transitions produce the correct bonded and gap segments.
func (s *KeeperSuite) TestProcessEvents_UnbondBondUnbond_ThreeTransitions() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 5 days. Unbond #1.
	tUnbond1 := t0.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tUnbond1).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.jailAndUnbondValidator(valAddr)

	evtUnbond1, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rateUnbond1 := evtUnbond1.TokensPerShare

	// Advance 10 days. Bond (unjail).
	tBond := tUnbond1.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tBond).WithBlockHeight(s.ctx.BlockHeight() + 1)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Unjail(s.ctx, consAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	// Advance 5 days. Unbond #2.
	tUnbond2 := tBond.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tUnbond2).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.jailAndUnbondValidator(valAddr)

	evtUnbond2, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(3)))
	s.Require().NoError(err)
	rateUnbond2 := evtUnbond2.TokensPerShare

	// Advance 10 days (second gap).
	tNow := tUnbond2.Add(10 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Expected: seg1[T0, T_unbond1] + zero gap + seg2[T_bond, T_unbond2] + zero gap
	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, t0, tUnbond1, rateUnbond1)
	seg2 := s.keeper.ComputeSegmentBonus(pos, tier, tBond, tUnbond2, rateUnbond2)
	expectedTotal := seg1.Add(seg2)

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must equal two bonded segments exactly (both gaps pay zero)")
}

// TestProcessEvents_MultipleSlashes_RateDecreases verifies that multiple
// slashes produce decreasing rates and correct segment sums.
func (s *KeeperSuite) TestProcessEvents_MultipleSlashes_RateDecreases() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 5 days. Slash 10%.
	tSlash1 := t0.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tSlash1).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))

	evtSlash1, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rate1 := evtSlash1.TokensPerShare

	// Advance 5 days. Slash 10% again.
	tSlash2 := tSlash1.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tSlash2).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))

	evtSlash2, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(2)))
	s.Require().NoError(err)
	rate2 := evtSlash2.TokensPerShare

	// Advance 5 more days.
	tNow := tSlash2.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	currentRate, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	// Verify rates are decreasing.
	s.Require().True(rate1.GT(rate2), "rate1 (%s) should be > rate2 (%s)", rate1, rate2)
	s.Require().True(rate2.GT(currentRate), "rate2 (%s) should be > currentRate (%s)", rate2, currentRate)

	// Expected: seg1[T0, T_slash1, rate1] + seg2[T_slash1, T_slash2, rate2] + seg3[T_slash2, T_now, currentRate]
	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, t0, tSlash1, rate1)
	seg2 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash1, tSlash2, rate2)
	seg3 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash2, tNow, currentRate)
	expectedTotal := seg1.Add(seg2).Add(seg3)

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must equal three segments exactly with decreasing rates")
}

// TestProcessEvents_AllThreeEventTypes_SlashUnbondBond verifies segmented bonus
// across all three event types in a single flow:
// create → 10d bonded → SLASH → 5d bonded → UNBOND → 20d gap → BOND → 10d bonded → claim
// Expected bonus = seg1[T0,T_slash] + seg2[T_slash,T_unbond] + seg3[T_bond,T_now]
// The 20d unbonded gap pays zero.
func (s *KeeperSuite) TestProcessEvents_AllThreeEventTypes_SlashUnbondBond() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	bondDenom, _ := s.app.StakingKeeper.BondDenom(s.ctx)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	tier, _ := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	T0 := pos.LastBonusAccrual

	// --- Segment 1: 10 days bonded, then SLASH ---
	s.ctx = s.ctx.WithBlockTime(T0.Add(10 * 24 * time.Hour))
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	T_slash := s.ctx.BlockTime()

	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(5, 2)) // 5% slash

	// Read the slash event's snapshot rate.
	evt1, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rate1 := evt1.TokensPerShare

	// --- Segment 2: 5 more days bonded, then UNBOND ---
	s.ctx = s.ctx.WithBlockTime(T_slash.Add(5 * 24 * time.Hour))
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	T_unbond := s.ctx.BlockTime()

	s.jailAndUnbondValidator(valAddr)

	evt2, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(2)))
	s.Require().NoError(err)
	rate2 := evt2.TokensPerShare

	// --- Gap: 20 days unbonded (zero bonus) ---
	s.ctx = s.ctx.WithBlockTime(T_unbond.Add(20 * 24 * time.Hour))
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// --- Segment 3: re-bond, then 10 days bonded ---
	val, _ := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	consAddr, _ := val.GetConsAddr()
	s.Require().NoError(s.app.StakingKeeper.Unjail(s.ctx, consAddr))
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)
	T_bond := s.ctx.BlockTime()

	s.ctx = s.ctx.WithBlockTime(T_bond.Add(10 * 24 * time.Hour))
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	T_now := s.ctx.BlockTime()

	// Current rate for final segment.
	currentRate, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	// --- Claim and verify ---
	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, T0, T_slash, rate1)
	seg2 := s.keeper.ComputeSegmentBonus(pos, tier, T_slash, T_unbond, rate2)
	seg3 := s.keeper.ComputeSegmentBonus(pos, tier, T_bond, T_now, currentRate)
	expectedTotal := seg1.Add(seg2).Add(seg3)

	s.Require().True(seg1.IsPositive(), "seg1 (pre-slash bonded) should be positive")
	s.Require().True(seg2.IsPositive(), "seg2 (post-slash bonded) should be positive")
	s.Require().True(seg3.IsPositive(), "seg3 (post-rebond bonded) should be positive")

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().Equal(expectedTotal.String(), bonus.AmountOf(bondDenom).String(),
		"bonus should equal seg1 + seg2 + seg3 (20d unbonded gap pays zero)")
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	// Verify decreasing rates: pre-slash > post-slash >= post-rebond.
	s.Require().True(rate1.GT(rate2), "rate should decrease after slash: rate1=%s, rate2=%s", rate1, rate2)

	// Second claim — zero.
	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	bonus2, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().True(bonus2.IsZero(), "second claim should be zero")
}

// ---------------------------------------------------------------------------
// ProcessEventsAndClaimBonus tests -- claim-at-different-junctures
// ---------------------------------------------------------------------------

// TestProcessEvents_ClaimBetweenEvents_NoDoubleClaim verifies that claiming
// between two slash events pays the correct segment for each claim, and a
// third claim at the same time yields zero.
func (s *KeeperSuite) TestProcessEvents_ClaimBetweenEvents_NoDoubleClaim() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 5 days. Slash.
	tSlash1 := t0.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tSlash1).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))

	evtSlash1, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rateSlash1 := evtSlash1.TokensPerShare

	// Claim #1: pays seg1 [T0, T_slash1] + tail segment (zero because tSlash1 == blockTime).
	currentRate1, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	seg1 := s.keeper.ComputeSegmentBonus(pos, tier, t0, tSlash1, rateSlash1)
	// At this point block time == tSlash1, so tail segment is [tSlash1, tSlash1] = zero.
	tailSeg1 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash1, tSlash1, currentRate1)
	expectedClaim1 := seg1.Add(tailSeg1)

	bonus1, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedClaim1.String(), bonus1.AmountOf(bondDenom).String(),
		"claim1 must equal seg1 exactly")

	// Advance 5 more days. Slash again.
	tSlash2 := tSlash1.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tSlash2).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))

	evtSlash2, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(2)))
	s.Require().NoError(err)
	rateSlash2 := evtSlash2.TokensPerShare

	// Claim #2: pays seg2 [T_slash1, T_slash2] only.
	currentRate2, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	seg2 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash1, tSlash2, rateSlash2)
	tailSeg2 := s.keeper.ComputeSegmentBonus(pos, tier, tSlash2, tSlash2, currentRate2)
	expectedClaim2 := seg2.Add(tailSeg2)

	bonus2, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedClaim2.String(), bonus2.AmountOf(bondDenom).String(),
		"claim2 must equal seg2 only (no double-counting seg1)")

	// Third claim at same time yields zero.
	bonus3, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))
	s.Require().True(bonus3.IsZero(), "third claim at same time should yield zero")
}

// TestProcessEvents_ClaimDuringUnbondGap_ThenClaimAfterRebond verifies the
// LastKnownBonded persistence: claiming during an unbond gap does not cause
// the post-rebond claim to include the gap period.
func (s *KeeperSuite) TestProcessEvents_ClaimDuringUnbondGap_ThenClaimAfterRebond() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 5 days. Unbond.
	tUnbond := t0.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tUnbond).WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.jailAndUnbondValidator(valAddr)

	evtUnbond, err := s.keeper.ValidatorEvents.Get(s.ctx, collections.Join(valAddr, uint64(1)))
	s.Require().NoError(err)
	rateAtUnbond := evtUnbond.TokensPerShare

	// Claim #1: pays pre-unbond segment.
	segPreUnbond := s.keeper.ComputeSegmentBonus(pos, tier, t0, tUnbond, rateAtUnbond)

	bonus1, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(segPreUnbond.String(), bonus1.AmountOf(bondDenom).String(),
		"claim1 must equal pre-unbond segment")
	s.Require().False(pos.LastKnownBonded,
		"LastKnownBonded should be false after processing UNBOND")

	// Advance 20 days (gap). Rebond.
	tBond := tUnbond.Add(20 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tBond).WithBlockHeight(s.ctx.BlockHeight() + 1)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Unjail(s.ctx, consAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	// Advance 5 days bonded.
	tNow := tBond.Add(5 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	currentRate, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	// Claim #2: should pay ONLY post-rebond segment, NOT the 20-day gap.
	segPostBond := s.keeper.ComputeSegmentBonus(pos, tier, tBond, tNow, currentRate)

	bonus2, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(segPostBond.String(), bonus2.AmountOf(bondDenom).String(),
		"claim2 must equal post-rebond segment only (no gap overpay)")
	s.Require().True(pos.LastKnownBonded,
		"LastKnownBonded should be true after processing BOND")
}

// TestProcessEvents_ClaimAfterExitUnlockAt_CappedBonus verifies that bonus
// is capped at ExitUnlockAt even when claiming well past it.
func (s *KeeperSuite) TestProcessEvents_ClaimAfterExitUnlockAt_CappedBonus() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true) // triggers exit immediately
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	t0 := pos.LastBonusAccrual
	exitUnlockAt := pos.ExitUnlockAt
	s.Require().False(exitUnlockAt.IsZero(), "exit should be triggered")

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// Advance 365 days past ExitUnlockAt.
	tNow := exitUnlockAt.Add(365 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(tNow).WithBlockHeight(s.ctx.BlockHeight() + 1)

	currentRate, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	// Expected: bonus capped at ExitUnlockAt.
	expectedBonus := s.keeper.ComputeSegmentBonus(pos, tier, t0, exitUnlockAt, currentRate)

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))

	s.Require().Equal(expectedBonus.String(), bonus.AmountOf(bondDenom).String(),
		"bonus must be capped at ExitUnlockAt")

	// Second claim yields zero.
	bonus2, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos.Position, nil))
	s.Require().True(bonus2.IsZero(), "second claim should yield zero")
}

// ---------------------------------------------------------------------------
// ProcessEventsAndClaimBonus tests -- edge cases
// ---------------------------------------------------------------------------

// TestProcessEvents_UndelegatedPosition_Zero verifies that an undelegated
// position returns zero bonus and no error.
func (s *KeeperSuite) TestProcessEvents_UndelegatedPosition_Zero() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	// Undelegate via msg server so the position is properly cleared.
	s.advancePastExitDuration()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfter.IsDelegated(), "position should be undelegated")

	bonus, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &posAfter)
	s.Require().NoError(err)
	s.Require().True(bonus.IsZero(), "bonus should be zero for undelegated position")
}

// TestProcessEvents_InsufficientPool_Error verifies that claiming without
// a funded pool returns ErrInsufficientBonusPool.
func (s *KeeperSuite) TestProcessEvents_InsufficientPool_Error() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)

	// Advance time so bonus would be non-zero. Do NOT fund the pool.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	_, err := s.keeper.ProcessEventsAndClaimBonus(s.ctx, &pos)
	s.Require().Error(err, "should fail when bonus pool is insufficient")
	s.Require().ErrorContains(err, "insufficient bonus pool",
		"error should mention insufficient bonus pool")
}
