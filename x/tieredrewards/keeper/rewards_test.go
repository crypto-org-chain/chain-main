package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------------------------------------------------------------------------
// Bonus rewards
// ---------------------------------------------------------------------------
// Claiming bonus while validator is bonded should yield positive bonus.
func (s *KeeperSuite) TestClaimBonusRewards_BondedValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// Calculate expected bonus BEFORE claiming (pos.LastBonusAccrual is still old)
	expectedBonus := s.keeper.CalculateBonusRaw(pos, val, tier, s.ctx.BlockTime())

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	expectedBonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, expectedBonus))

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, pos.Owner, val, tier, false)
	s.Require().NoError(err)
	s.Require().Equal(expectedBonusCoins, bonus)
}

// AfterValidatorBeginUnbonding settles the final bonus (forceAccrue) and
// advances LastBonusAccrual to block time. Subsequent claims see the
// validator as unbonding and calculateBonus returns zero.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_SettlesFinalBonus() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	unbondTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(unbondTime)

	s.jailAndUnbondValidator(valAddr)

	// LastBonusAccrual should be advanced to the block time (not zeroed).
	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(unbondTime, updated.LastBonusAccrual,
		"LastBonusAccrual should be advanced to block time after unbonding hook")
}

// If the hook tolerates an empty bonus pool, it must still persist the advanced
// checkpoint so the pre-unbond accrual window is not repriced later.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_InsufficientBonusPoolAdvancesCheckpoint() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	unbondTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(unbondTime)

	s.jailAndUnbondValidator(valAddr)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(unbondTime, updated.LastBonusAccrual,
		"LastBonusAccrual should advance even when bonus pool is empty during unbonding hook")
}

// MsgClaimTierRewards returns zero bonus when the validator is not bonded.
func (s *KeeperSuite) TestClaimTierRewards_UnbondingValidator_ZeroBonus() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Jail + apply → unbonding (hook settles final bonus).
	s.jailAndUnbondValidator(valAddr)

	// Advance time further; the validator is still unbonding.
	claimTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(claimTime)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	s.Require().True(resp.BonusRewards.IsZero(),
		"bonus should be zero for an unbonding validator; got %s", resp.BonusRewards)

	// LastBonusAccrual should advance to current block time.
	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(claimTime, updated.LastBonusAccrual,
		"LastBonusAccrual should advance to block time even when bonus is zero")
}

// After the validator re-bonds, bonus accrual should resume from the new bonded time.
func (s *KeeperSuite) TestBonusAccrual_ResumesAfterRebond() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Jail + apply → unbonding (hook settles final bonus).
	s.jailAndUnbondValidator(valAddr)

	// Verify LastBonusAccrual was advanced (not zeroed).
	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
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

	afterBonded, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(s.ctx.BlockTime(), afterBonded.LastBonusAccrual)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	updated, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	expectedBonus := s.keeper.CalculateBonusRaw(updated, val, tier, s.ctx.BlockTime())
	expectedBonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, expectedBonus))

	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &updated, pos.Owner, val, tier, false)
	s.Require().NoError(err)
	s.Require().Equal(expectedBonusCoins, bonus)

	s.Require().Equal(s.ctx.BlockTime(), updated.LastBonusAccrual)
}

// calculateBonus returns zero when the validator is not bonded.
func (s *KeeperSuite) TestCalculateBonus_UnbondedValidator_ReturnsZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	s.jailAndUnbondValidator(valAddr)

	// Advance time so there would be a non-zero bonus if the validator were bonded.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Re-read position (hook advanced LastBonusAccrual).
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Re-read validator after jailing so IsBonded() reflects current status.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, pos.Owner, val, tier, false)
	s.Require().NoError(err)
	s.Require().True(bonus.Empty(),
		"bonus should be empty when validator is not bonded; got %s", bonus)
}

// forceAccrue=true still yields bonus even when the validator is not bonded.
func (s *KeeperSuite) TestClaimBonusRewards_ForceAccrue() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	s.jailAndUnbondValidator(valAddr)

	// Advance time.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// forceAccrue=true → calculateBonusRaw → ignores validator status.
	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, pos.Owner, val, tier, true)
	s.Require().NoError(err)
	s.Require().False(bonus.IsZero(),
		"forceAccrue=true should yield bonus even for an unbonded validator")
}

// TestSettleRewardsForPositions_UpdatesOriginalSlice verifies:
// SettleRewardsForPositions must update the caller's slice elements in-place
// (pointer semantics) so callers that hold the slice after the call see updated
// LastBonusAccrual values — preventing double-claim of bonus rewards.
func (s *KeeperSuite) TestSettleRewardsForPositions_UpdatesOriginalSlice() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 30))

	positions, err := s.keeper.GetPositionsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	originalLastAccrual := positions[0].LastBonusAccrual

	// claims both base and bonus rewards
	_, err = s.keeper.SettleRewardsForPositions(s.ctx, valAddr, positions, false)
	s.Require().NoError(err)

	// After the call the slice element must reflect the updated LastBonusAccrual.
	s.Require().NotEqual(originalLastAccrual, positions[0].LastBonusAccrual,
		"SettleRewardsForPositions must update the slice element in-place")

	// Also confirm the store is in sync.
	stored, err := s.keeper.GetPosition(s.ctx, positions[0].Id)
	s.Require().NoError(err)
	s.Require().Equal(positions[0].LastBonusAccrual, stored.LastBonusAccrual,
		"in-memory slice element must match the stored position")
}

// TestSettleRewardsForPositions_MixedInsufficientBonusPool verifies that the
// fused reward-settlement loop can successfully pay earlier positions, then
// persist checkpoints for a later position that hits ErrInsufficientBonusPool.
func (s *KeeperSuite) TestSettleRewardsForPositions_MixedInsufficientBonusPool() {
	lockAmount1 := sdkmath.NewInt(10_000)
	lockAmount2 := sdkmath.NewInt(20_000)
	pos1 := s.setupNewTierPosition(lockAmount1, false)
	pos2 := s.setupNewTierPosition(lockAmount2, false)
	addr1 := sdk.MustAccAddressFromBech32(pos1.Owner)
	addr2 := sdk.MustAccAddressFromBech32(pos2.Owner)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	positions1, err := s.keeper.GetPositionsByOwner(s.ctx, addr1)
	s.Require().NoError(err)
	s.Require().Len(positions1, 1)
	pos1 = positions1[0]

	positions2, err := s.keeper.GetPositionsByOwner(s.ctx, addr2)
	s.Require().NoError(err)
	s.Require().Len(positions2, 1)
	pos2 = positions2[0]

	tier, err := s.keeper.GetTier(s.ctx, pos1.TierId)
	s.Require().NoError(err)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	bonus1 := s.keeper.CalculateBonusRaw(pos1, val, tier, s.ctx.BlockTime())
	bonus2 := s.keeper.CalculateBonusRaw(pos2, val, tier, s.ctx.BlockTime())

	s.Require().True(bonus1.IsPositive())
	s.Require().True(bonus2.IsPositive())

	s.fundRewardsPool(bonus1, bondDenom)

	bal1Before := s.app.BankKeeper.GetBalance(s.ctx, addr1, bondDenom)
	bal2Before := s.app.BankKeeper.GetBalance(s.ctx, addr2, bondDenom)
	positions := []types.Position{pos1, pos2}

	_, err = s.keeper.SettleRewardsForPositions(s.ctx, valAddr, positions, false)
	s.Require().NoError(err)

	bal1After := s.app.BankKeeper.GetBalance(s.ctx, addr1, bondDenom)
	bal2After := s.app.BankKeeper.GetBalance(s.ctx, addr2, bondDenom)
	s.Require().Equal(bal1Before.Amount.Add(bonus1).String(), bal1After.Amount.String(),
		"first owner should receive the paid bonus")
	s.Require().Equal(bal2Before.Amount.String(), bal2After.Amount.String(),
		"second owner should not receive bonus when the pool is exhausted")

	for _, pos := range positions {
		s.Require().Equal(s.ctx.BlockTime(), pos.LastBonusAccrual,
			"in-memory position checkpoint should still advance on insufficient bonus pool")
		stored, err := s.keeper.GetPosition(s.ctx, pos.Id)
		s.Require().NoError(err)
		s.Require().Equal(s.ctx.BlockTime(), stored.LastBonusAccrual,
			"stored position checkpoint should still advance on insufficient bonus pool")
	}
}

// TestClaimBonusRewards_UsesIntegerDivisionForDuration verifies:
// the bonus duration is computed with integer division (not float64.Seconds()),
// so there is no truncation bias from float representation.
// We construct a duration that has sub-second remainder and confirm the result
// matches integer division.
func (s *KeeperSuite) TestClaimBonusRewards_DurationUsesIntegerSeconds() {
	const lockAmt = 1_000_000_000 // large enough for measurable bonus
	pos := s.setupNewTierPosition(sdkmath.NewInt(lockAmt), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	// Advance by a duration that is NOT an exact number of seconds
	// (1h + 500ms). The bonus should be computed for exactly 3600 seconds,
	// not 3600.5 seconds.
	advanceDuration := time.Hour + 500*time.Millisecond
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(advanceDuration))
	s.fundRewardsPool(sdkmath.NewInt(10_000_000_000), bondDenom)

	posNow, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, posNow.TierId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// Compute expected bonus using integer division.
	expectedBonus := s.keeper.CalculateBonusRaw(posNow, val, tier, s.ctx.BlockTime())

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().True(expectedBonus.Equal(resp.BonusRewards.AmountOf(bondDenom)),
		"bonus should match integer-second calculation, got %s expected %s",
		resp.BonusRewards.AmountOf(bondDenom), expectedBonus)
}

// TestCalculateBonusRaw_ZeroAmount verifies that bonus is zero when the
// position has zero delegated shares (e.g. after 100% slash on redelegation).
func (s *KeeperSuite) TestCalculateBonusRaw_ZeroAmount() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, uint32(1))
	s.Require().NoError(err)

	blockTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)

	// Position with zero shares (100% slash on redelegation).
	pos := types.Position{
		Amount:           sdkmath.ZeroInt(),
		DelegatedShares:  sdkmath.LegacyZeroDec(),
		Validator:        "",
		LastBonusAccrual: s.ctx.BlockTime(),
	}
	bonus := s.keeper.CalculateBonusRaw(pos, val, tier, blockTime)
	s.Require().True(bonus.IsZero(),
		"bonus should be zero for undelegated position with zero shares")
}

// TestCalculateBonusRaw_SharesWorthless verifies that bonus is zero when
// the position has non-zero delegated shares but the validator has been
// slashed to zero tokens, making TokensFromShares return zero.
func (s *KeeperSuite) TestCalculateBonusRaw_SharesWorthless() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.Require().True(pos.DelegatedShares.IsPositive(), "should have shares")

	// Slash validator to zero — shares remain but are now worthless.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(val.GetTokens().IsZero(),
		"validator tokens should be zero after 100%% slash")

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	blockTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)

	// TokensFromShares returns zero because validator has no tokens.
	tokens := val.TokensFromShares(pos.DelegatedShares)
	s.Require().True(tokens.IsZero(),
		"tokens from shares should be zero for slashed validator")

	bonus := s.keeper.CalculateBonusRaw(pos, val, tier, blockTime)
	s.Require().True(bonus.IsZero(),
		"bonus should be zero when shares are worthless (validator slashed to zero)")

	// Also verify via ClaimBonusRewards.
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	claimed, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, pos.Owner, val, tier, false)
	s.Require().NoError(err)
	s.Require().True(claimed.IsZero(),
		"claimed bonus should be zero when shares are worthless")
}

// TestCalculateBonusRaw_ZeroLastBonusAccrual verifies that bonus is zero
// when LastBonusAccrual is the zero time.
func (s *KeeperSuite) TestCalculateBonusRaw_ZeroLastBonusAccrual() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, uint32(1))
	s.Require().NoError(err)

	pos := types.Position{
		Amount:          sdkmath.NewInt(10000),
		DelegatedShares: sdkmath.LegacyNewDec(10000),
		Validator:       valAddr.String(),
		// LastBonusAccrual is zero time.
	}
	bonus := s.keeper.CalculateBonusRaw(pos, val, tier, s.ctx.BlockTime().Add(30*24*time.Hour))
	s.Require().True(bonus.IsZero(),
		"bonus should be zero when LastBonusAccrual is zero time")
}

// TestCalculateBonus_StopsAccruingAfterExitUnlockAt verifies that bonus
// accrual is capped at ExitUnlockAt when the position has completed its
// exit lock duration. Advancing time further past ExitUnlockAt should
// not yield additional bonus.
func (s *KeeperSuite) TestCalculateBonus_StopsAccruingAfterExitUnlockAt() {
	const lockAmt = 1_000_000_000
	pos := s.setupNewTierPosition(sdkmath.NewInt(lockAmt), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()

	exitUnlockAt := pos.ExitUnlockAt

	// Claim rewards exactly at ExitUnlockAt.
	s.ctx = s.ctx.WithBlockTime(exitUnlockAt)
	s.fundRewardsPool(sdkmath.NewInt(100_000_000_000), bondDenom)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	respAtUnlock, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	bonusAtUnlock := respAtUnlock.BonusRewards.AmountOf(bondDenom)

	// Advance time well past ExitUnlockAt — bonus should not increase.
	s.ctx = s.ctx.WithBlockTime(exitUnlockAt.Add(time.Hour * 24 * 365))

	respAfterUnlock, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	bonusAfterUnlock := respAfterUnlock.BonusRewards.AmountOf(bondDenom)

	s.Require().True(bonusAtUnlock.IsPositive(), "should have accrued bonus up to ExitUnlockAt")
	s.Require().True(bonusAfterUnlock.IsZero(),
		"bonus should not accrue past ExitUnlockAt, got %s", bonusAfterUnlock)
}

// ---------------------------------------------------------------------------
// Base rewards
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestBaseRewardsWithdrawal_MarkedOncePerBlock() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	currentHeight := uint64(s.ctx.BlockHeight())

	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	lastWithdrawalBlock := s.keeper.GetLastWithdrawalBlock(s.ctx, valAddr)
	s.Require().Equal(currentHeight, lastWithdrawalBlock)

	_, err = msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	lastWithdrawalBlock = s.keeper.GetLastWithdrawalBlock(s.ctx, valAddr)
	s.Require().Equal(currentHeight, lastWithdrawalBlock, "same-block claim should keep withdrawal marker unchanged")

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	nextHeight := uint64(s.ctx.BlockHeight())

	_, err = msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	lastWithdrawalBlock = s.keeper.GetLastWithdrawalBlock(s.ctx, valAddr)
	s.Require().Equal(nextHeight, lastWithdrawalBlock, "new block should update withdrawal marker")
}

// TestBaseRewardsPerShare_UpdatesOnClaim verifies that after claiming,
// the position's BaseRewardsPerShare matches the validator's current ratio.
func (s *KeeperSuite) TestBaseRewardsPerShare_UpdatesOnClaim() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	ratioBefore := pos.BaseRewardsPerShare

	// Allocate rewards and advance block so collectDelegationRewards fires.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Position's checkpoint should have advanced.
	diff, hasNeg := pos.BaseRewardsPerShare.SafeSub(ratioBefore)
	s.Require().False(hasNeg, "BaseRewardsPerShare should not decrease")
	s.Require().False(diff.IsZero(),
		"BaseRewardsPerShare should increase after claim: before=%s, after=%s",
		ratioBefore, pos.BaseRewardsPerShare)

	// Position checkpoint should equal the validator's current ratio.
	validatorRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(validatorRatio, pos.BaseRewardsPerShare,
		"position checkpoint should match validator ratio after claim")
}

// TestBaseRewardsPerShare_UnchangedWhenUndelegated verifies that an
// undelegated position's BaseRewardsPerShare does not change even as the
// validator's ratio keeps increasing.
func (s *KeeperSuite) TestBaseRewardsPerShare_UnchangedWhenUndelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance past exit and undelegate.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Snapshot the undelegated position's checkpoint.
	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	ratioSnapshot := posAfter.BaseRewardsPerShare

	// Allocate more rewards to the validator and advance blocks.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(5000), bondDenom)

	// Claim on the undelegated position — should be a no-op for base rewards.
	_, err = msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	posAfter, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(ratioSnapshot, posAfter.BaseRewardsPerShare,
		"undelegated position's BaseRewardsPerShare should not change")
}

// TestValidatorRewardRatio_IncreasesEachBlock verifies that the validator's
// cumulative reward ratio grows with each block when rewards are allocated.
func (s *KeeperSuite) TestValidatorRewardRatio_IncreasesEachBlock() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	var prevRatio sdk.DecCoins

	for block := int64(1); block <= 3; block++ {
		s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
		s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

		_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
			Owner:       delAddr.String(),
			PositionIds: []uint64{pos.Id},
		})
		s.Require().NoError(err)

		ratio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
		s.Require().NoError(err)

		if block > 1 {
			diff, hasNeg := ratio.SafeSub(prevRatio)
			s.Require().False(hasNeg, "ratio should not decrease")
			s.Require().False(diff.IsZero(),
				"ratio should increase: block=%d, prev=%s, curr=%s",
				block, prevRatio, ratio)
		}
		prevRatio = ratio
	}
}

// TestClaimBaseRewards_CorrectAmount verifies the base reward payout matches
// the formula: DelegatedShares * (currentRatio - positionRatio).
func (s *KeeperSuite) TestClaimBaseRewards_CorrectAmount() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	posRatio := pos.BaseRewardsPerShare

	// Allocate known rewards and advance block.
	rewardAmount := sdkmath.NewInt(500)
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, rewardAmount, bondDenom)

	rsp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	// Compute expected: shares * (newRatio - oldRatio), truncated.
	newRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)

	delta, _ := newRatio.SafeSub(posRatio)
	expectedDec := delta.MulDecTruncate(pos.DelegatedShares)
	expected, _ := expectedDec.TruncateDecimal()
	expectedBaseAmount := expected.AmountOf(bondDenom)

	s.Require().True(expectedBaseAmount.IsPositive(),
		"expected base reward should be positive")
	s.Require().Equal(rsp.BaseRewards.AmountOf(bondDenom), expectedBaseAmount)
}

// TestClaimBaseRewards_ZeroDelta verifies that when the validator ratio
// has not changed since the position's checkpoint, no base rewards are paid.
func (s *KeeperSuite) TestClaimBaseRewards_ZeroDelta() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Claim once to sync position checkpoint with validator ratio.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner: delAddr.String(), PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	// Don't allocate any new rewards. Claim again on same block —
	// ratio should be unchanged, base reward should be zero.
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	rsp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner: delAddr.String(), PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().True(rsp.BaseRewards.IsZero(),
		"base rewards should be zero when ratio is unchanged")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	// Only bonus (if any) should be received, base should be zero.
	// With zero time elapsed since last claim, bonus should also be zero.
	s.Require().Equal(balBefore.Amount, balAfter.Amount,
		"balance should not change when no new rewards accrued")
}

// TestClaimBaseRewards_UndelegatedPosition verifies that claimBaseRewards
// returns empty coins for an undelegated position.
func (s *KeeperSuite) TestClaimBaseRewards_UndelegatedPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	vals, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfter.IsDelegated(), "position should be undelegated")

	// Direct call to ClaimBaseRewards with any ratio — should return empty.
	currentRatio := sdk.NewDecCoins(sdk.NewDecCoin(bondDenom, sdkmath.NewInt(999)))
	base, err := s.keeper.ClaimBaseRewards(s.ctx, []*types.Position{&posAfter}, delAddr.String(), sdk.MustValAddressFromBech32(vals[0].GetOperator()), currentRatio)
	s.Require().NoError(err)
	s.Require().True(base.IsZero(),
		"claimBaseRewards on undelegated position should return empty")
}

// TestUpdateBaseRewardsPerShare_NoDelegation verifies that when the module
// has no delegation to a validator, updateBaseRewardsPerShare returns empty.
func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_NoDelegation() {
	// Use a validator address that the module has never delegated to.
	fakeVal := sdk.ValAddress([]byte("no_delegation_val___"))

	ratio, err := s.keeper.UpdateBaseRewardsPerShare(s.ctx, fakeVal)
	s.Require().NoError(err)
	s.Require().True(ratio.IsZero(),
		"ratio should be empty when no delegation exists")
}

// TestUpdateBaseRewardsPerShare_ZeroShares verifies that if the module's
// delegation has zero shares, the ratio is not updated.
func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_ZeroShares() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Undelegate to remove the module's delegation shares.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	ratio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(500), bondDenom)

	// With no module delegation, updateBaseRewardsPerShare returns empty or
	// the existing ratio without updating.
	updatedRatio, err := s.keeper.UpdateBaseRewardsPerShare(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(ratio, updatedRatio)
}

// TestClaimRewardsForPosition_Undelegated verifies that claimRewardsForPosition
// returns early with zero rewards for an undelegated position.
func (s *KeeperSuite) TestClaimRewardsForPosition_Undelegated() {
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

	pos, err := s.keeper.GetPosition(s.ctx, posSetup.Id)
	s.Require().NoError(err)

	updated, base, bonus, err := s.keeper.ClaimRewardsForPosition(s.ctx, pos)
	s.Require().NoError(err)
	s.Require().True(base.IsZero(), "base should be zero for undelegated position")
	s.Require().True(bonus.IsZero(), "bonus should be zero for undelegated position")
	s.Require().True(pos.BaseRewardsPerShare.IsZero(), "base rewards per share should be zero for undelegated position")
	s.Require().Equal(pos.BaseRewardsPerShare, updated.BaseRewardsPerShare,
		"base rewards per share should not change for undelegated position")
}

// TestClaimRewardsAndUpdatePositionsForTier_ClaimsAll verifies that the
// tier-sweep function claims rewards for all delegated positions in a tier.
func (s *KeeperSuite) TestClaimRewardsAndUpdatePositionsForTier_ClaimsAll() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos1 := s.setupNewTierPosition(lockAmount, false)
	addr1 := sdk.MustAccAddressFromBech32(pos1.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos1.Validator)
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
	err := s.keeper.ClaimRewardsAndUpdatePositionsForTier(s.ctx, 1)
	s.Require().NoError(err)

	// Both owners should receive rewards.
	bal1After := s.app.BankKeeper.GetBalance(s.ctx, addr1, bondDenom)
	bal2After := s.app.BankKeeper.GetBalance(s.ctx, addr2, bondDenom)
	s.Require().True(bal1After.Amount.GT(bal1Before.Amount),
		"addr1 should receive rewards from tier sweep")
	s.Require().True(bal2After.Amount.GT(bal2Before.Amount),
		"addr2 should receive rewards from tier sweep")

	// Both positions' BaseRewardsPerShare should be updated to the current ratio.
	ratio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)

	posA, err := s.keeper.GetPosition(s.ctx, pos1.Id)
	s.Require().NoError(err)
	positions2, err := s.keeper.GetPositionsByOwner(s.ctx, addr2)
	s.Require().NoError(err)
	s.Require().Len(positions2, 1)
	posB := positions2[0]

	s.Require().Equal(ratio, posA.BaseRewardsPerShare,
		"pos0 checkpoint should match validator ratio after tier sweep")
	s.Require().Equal(ratio, posB.BaseRewardsPerShare,
		"pos1 checkpoint should match validator ratio after tier sweep")
}

// TestClaimRewardsAndUpdatePositionsForTier_SkipsUndelegated verifies that
// the tier-sweep skips undelegated positions.
func (s *KeeperSuite) TestClaimRewardsAndUpdatePositionsForTier_SkipsUndelegated() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	// Position 0: delegated.
	pos0 := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos0.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Position 1: will be undelegated.
	pos1 := s.setupNewTierPosition(lockAmount, true)
	addr2 := sdk.MustAccAddressFromBech32(pos1.Owner)

	pos1Positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr2)
	s.Require().NoError(err)
	s.Require().Len(pos1Positions, 1)
	pos1Id := pos1Positions[0].Id

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: addr2.String(), PositionId: pos1Id,
	})
	s.Require().NoError(err)

	pos1Before, err := s.keeper.GetPosition(s.ctx, pos1Id)
	s.Require().NoError(err)
	s.Require().False(pos1Before.IsDelegated())

	// Allocate rewards and run tier sweep.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	err = s.keeper.ClaimRewardsAndUpdatePositionsForTier(s.ctx, 1)
	s.Require().NoError(err)

	// Undelegated position should not have its checkpoint updated.
	pos1After, err := s.keeper.GetPosition(s.ctx, pos1Id)
	s.Require().NoError(err)
	s.Require().True(pos1After.BaseRewardsPerShare.IsZero(), "base rewards per share should be zero for undelegated position")
	s.Require().Equal(pos1Before.BaseRewardsPerShare, pos1After.BaseRewardsPerShare,
		"undelegated position checkpoint should not change during tier sweep")
}

// TestStaleValidatorRewardRatioReplayed verifies stale validator
// ratio is cleared when module delegation on that validator reaches zero, so a
// later delegation lifecycle cannot replay historical base rewards.
func (s *KeeperSuite) TestStaleValidatorRewardRatioReplayed() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// First lifecycle: claim rewards to leave a non-zero validator ratio.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	ratioAfterFirstClaim, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(ratioAfterFirstClaim.IsZero(), "test setup failed: expected a non-zero ratio after first claim")

	// Fully close the first position so module delegation on the validator is removed.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr)

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().Error(err, "expected no module delegation after position withdrawal")

	staleRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(staleRatio.IsZero(), "test setup failed: expected historical ratio before re-entry")

	// Second lifecycle: create a fresh position with no new rewards allocated.
	pos2 := s.setupNewTierPosition(lockAmount, false)
	addr2 := sdk.MustAccAddressFromBech32(pos2.Owner)

	ratioAfterReentry, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ratioAfterReentry.IsZero(), "stale validator ratio should be reset on re-entry when no module delegation exists")

	// No new rewards were allocated for this second lifecycle.
	claimResp2, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr2.String(),
		PositionIds: []uint64{pos2.Id},
	})
	s.Require().NoError(err)

	s.Require().True(
		claimResp2.BaseRewards.AmountOf(bondDenom).IsZero(),
		"second lifecycle claim should not replay historical base rewards when no new rewards were allocated",
	)
}
