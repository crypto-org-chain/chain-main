package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// --- BeforeValidatorSlashed hook tests ---

// TestBeforeValidatorSlashed_ClaimsRewardsBeforeSlash verifies that the hook
// claims pending base rewards before applying the slash, so the position's
// BaseRewardsPerShare snapshot is updated and cannot be double-claimed later.
func (s *KeeperSuite) TestBeforeValidatorSlashed_ClaimsRewardsBeforeSlash() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Trigger the slash hook directly with a tiny fraction so positions are affected.
	hooks := s.keeper.Hooks()
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2)) // 1% slash
	s.Require().NoError(err)

	// Owner should have received base (and bonus) rewards that were settled before the slash.
	balAfterSlash := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterSlash.Amount.GT(balBefore.Amount), "rewards should have been paid before slash")

	// Calling ClaimTierRewards now should not pay base rewards again (already claimed in hook).
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	respClaim, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(respClaim.BaseRewards.IsZero(), "base rewards should already be claimed by the slash hook")

	balAfterClaim := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(balAfterSlash.Amount, balAfterClaim.Amount, "second claim should yield nothing extra")
}

// TestBeforeValidatorSlashed_ReducesPositionAmount verifies that the hook no
// longer rewrites position.Amount eagerly.
func (s *KeeperSuite) TestBeforeValidatorSlashed_ReducesPositionAmount() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	slashFraction := sdkmath.LegacyMustNewDecFromStr("0.333333333333333333")

	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.Equal(valBefore.TokensFromShares(posBefore.DelegatedShares).TruncateInt()),
		"position amount should remain unchanged until staking slash updates validator exchange rate")
}

// TestBeforeValidatorSlashed_DoesNotRevertLastBonusAccrual verifies:
// after the hook, the position's LastBonusAccrual in the store reflects the time
// the rewards were settled, not the pre-claim value.
func (s *KeeperSuite) TestBeforeValidatorSlashed_DoesNotRevertLastBonusAccrual() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	claimTime := s.ctx.BlockTime().Add(time.Hour * 24)
	s.ctx = s.ctx.WithBlockTime(claimTime)

	hooks := s.keeper.Hooks()
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// LastBonusAccrual must be updated to claimTime, not left at the initial block time.
	s.Require().Equal(claimTime, posAfter.LastBonusAccrual, "LastBonusAccrual must be updated after slash hook")
}

// TestBeforeValidatorSlashed_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestBeforeValidatorSlashed_NoPositions() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	hooks := s.keeper.Hooks()

	// Should not error when there are no positions.
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)
}

// TestBeforeValidatorSlashed_InsufficientBonusPool verifies the ErrInsufficientBonusPool
// path: when the bonus pool cannot cover the accrued bonus, the hook still completes
// without error and BaseRewardsPerShare is updated
// (base rewards were claimed even though bonus was not paid).
func (s *KeeperSuite) TestBeforeValidatorSlashed_InsufficientBonusPool() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance time so bonus accrues, but intentionally do NOT fund the bonus pool.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 365))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(200), bondDenom)
	// Bonus pool remains at 0 — any bonus claim will fail with ErrInsufficientBonusPool.

	posBefore, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	oldRatio := posBefore.BaseRewardsPerShare
	claimTime := s.ctx.BlockTime()

	slashFraction := sdkmath.LegacyNewDecWithPrec(5, 2) // 5%
	hooks := s.keeper.Hooks()

	// Hook must succeed even though bonus pool is empty.
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err, "BeforeValidatorSlashed must not fail when bonus pool is empty")

	posAfter, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)

	s.Require().True(posAfter.Amount.Equal(posBefore.Amount),
		"position amount should remain unchanged until staking slash updates validator exchange rate")

	// Base rewards must still be claimed (BaseRewardsPerShare updated).
	s.Require().NotEqual(oldRatio, posAfter.BaseRewardsPerShare,
		"BaseRewardsPerShare must be updated even when bonus pool is empty")
	s.Require().Equal(claimTime, posAfter.LastBonusAccrual,
		"LastBonusAccrual must still advance when bonus pool is empty")
}

// TestBeforeValidatorSlashed_FullSlash_DoesNotHaltChain verifies that a 100% slash
// hook call still succeeds without eagerly mutating position amount.
func (s *KeeperSuite) TestBeforeValidatorSlashed_FullSlash_DoesNotHaltChain() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	hooks := s.keeper.Hooks()
	// 100% slash — must not error even though position.Amount reaches zero.
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyOneDec())
	s.Require().NoError(err, "100% slash should not cause an error")

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.Equal(pos.Amount),
		"position amount should remain unchanged until staking slash updates validator exchange rate")
}

// TestBeforeValidatorSlashed_MultiplePositions verifies that the slash hook can
// process multiple positions without eagerly rewriting their amounts.
func (s *KeeperSuite) TestBeforeValidatorSlashed_MultiplePositions() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Fund the owner with enough tokens for 2 more LockTier calls.
	_, bondDenom := s.getStakingData()
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount.MulRaw(2))))
	s.Require().NoError(err)

	// Create two more positions on the same validator (first was created by setupNewTierPosition).
	for range 2 {
		_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
			Owner:            delAddr.String(),
			Id:               1,
			Amount:           lockAmount,
			ValidatorAddress: valAddr.String(),
		})
		s.Require().NoError(err)
	}

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 3)

	amountsBefore := make([]sdkmath.Int, 3)
	for i, p := range positions {
		amountsBefore[i] = p.Amount
	}

	slashFraction := sdkmath.LegacyNewDecWithPrec(10, 2) // 10%
	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	for i, p := range positions {
		posAfter, err := s.keeper.GetPosition(s.ctx, p.Id)
		s.Require().NoError(err)
		s.Require().True(posAfter.Amount.Equal(amountsBefore[i]),
			"position %d amount should remain unchanged before lazy reconciliation", p.Id)
	}
}

// TestBeforeValidatorSlashed_MultiplePositions_InsufficientBonusPool verifies
// that the hook can reuse the in-memory positions updated during reward
// settlement, without reloading from store, while still advancing checkpoints.
func (s *KeeperSuite) TestBeforeValidatorSlashed_MultiplePositions_InsufficientBonusPool() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	s.setupNewTierPosition(lockAmount, false)
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 365))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(200), bondDenom)

	pos0Before, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	pos1Before, err := s.keeper.GetPosition(s.ctx, uint64(1))
	s.Require().NoError(err)
	oldRatio0 := pos0Before.BaseRewardsPerShare
	oldRatio1 := pos1Before.BaseRewardsPerShare
	claimTime := s.ctx.BlockTime()

	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(5, 2))
	s.Require().NoError(err, "slash hook must tolerate insufficient bonus pool across multiple positions")

	pos0After, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	pos1After, err := s.keeper.GetPosition(s.ctx, uint64(1))
	s.Require().NoError(err)

	for _, pos := range []types.Position{pos0After, pos1After} {
		s.Require().Equal(claimTime, pos.LastBonusAccrual,
			"checkpoint must advance even when bonus payout fails")
	}
	s.Require().NotEqual(oldRatio0, pos0After.BaseRewardsPerShare,
		"first position base rewards snapshot should be updated before slash")
	s.Require().NotEqual(oldRatio1, pos1After.BaseRewardsPerShare,
		"second position base rewards snapshot should be updated before slash")
	s.Require().True(pos0After.Amount.Equal(pos0Before.Amount),
		"first position amount should remain unchanged before lazy reconciliation")
	s.Require().True(pos1After.Amount.Equal(pos1Before.Amount),
		"second position amount should remain unchanged before lazy reconciliation")
}

// TestBeforeValidatorSlashed_UpdatesPoolDelegationInfo verifies that the hook
// reduces the distribution module's DelegatorStartingInfo.Stake for the tier
// module pool by the slash fraction. Without this, the next reward claim would
// compare an outdated pre-slash stake with the lower post-slash stake, causing
// a negative-rewards panic or incorrect payout.
func (s *KeeperSuite) TestBeforeValidatorSlashed_UpdatesPoolDelegationInfo() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)

	// Read starting info before slash.
	infoBefore, err := s.app.DistrKeeper.GetDelegatorStartingInfo(s.ctx, valAddr, poolAddr)
	s.Require().NoError(err)
	s.Require().True(infoBefore.Stake.IsPositive(), "pool should have positive starting stake before slash")

	slashFraction := sdkmath.LegacyNewDecWithPrec(25, 2) // 25%
	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	// Read starting info after slash.
	infoAfter, err := s.app.DistrKeeper.GetDelegatorStartingInfo(s.ctx, valAddr, poolAddr)
	s.Require().NoError(err)

	expectedStake := infoBefore.Stake.MulTruncate(sdkmath.LegacyOneDec().Sub(slashFraction))
	s.Require().True(infoAfter.Stake.Equal(expectedStake),
		"starting stake should be reduced by slash fraction; got %s want %s",
		infoAfter.Stake, expectedStake)
}

// TestBeforeValidatorSlashed_LazyAmountAccounting verifies that bonded slash no
// longer rewrites stored position.Amount eagerly, and Amount is reconciled lazily
// from shares after validator exchange rate changes.
func (s *KeeperSuite) TestBeforeValidatorSlashed_LazyAmountAccounting() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	posBefore, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	fraction := sdkmath.LegacyNewDecWithPrec(10, 2) // 10%
	expectedAmount := valBefore.TokensFromShares(posBefore.DelegatedShares).Mul(sdkmath.LegacyOneDec().Sub(fraction)).TruncateInt()

	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, fraction)
	s.Require().NoError(err)

	// Raw stored position keeps its pre-slash amount until it is read/mutated.
	storedAfter, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(storedAfter.Amount.Equal(posBefore.Amount),
		"stored amount should remain unchanged before lazy reconciliation")

	// Simulate staking slash updating validator exchange rate.
	s.slashValidatorDirect(valAddr, fraction)

	// Public position reads reconcile from current validator share exchange rate.
	reconciledAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(reconciledAfter.Amount.Equal(expectedAmount),
		"reconciled amount should match post-slash token value")
}

// --- AfterValidatorBonded hook tests ---

func (s *KeeperSuite) recordUnbondingCheckpoint(valAddr sdk.ValAddress) sdk.ConsAddress {
	consAddr := sdk.ConsAddress(valAddr)
	err := s.keeper.Hooks().AfterValidatorBeginUnbonding(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)
	return consAddr
}

func (s *KeeperSuite) recordBondedCheckpointAt(valAddr sdk.ValAddress, consAddr sdk.ConsAddress, at time.Time) {
	s.ctx = s.ctx.WithBlockTime(at)
	err := s.keeper.Hooks().AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)
}

// TestAfterValidatorBonded_RecordsResumeCheckpoint verifies that the hook stores
// a validator-level resume checkpoint without eagerly mutating all positions.
func (s *KeeperSuite) TestAfterValidatorBonded_RecordsResumeCheckpoint() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Bonded checkpoint only makes sense after a pause checkpoint exists.
	consAddr := s.recordUnbondingCheckpoint(valAddr)

	// Simulate passage of time.
	newTime := s.ctx.BlockTime().Add(time.Hour * 48)
	s.recordBondedCheckpointAt(valAddr, consAddr, newTime)

	resumeAt, ok, err := s.keeper.GetValidatorBonusResumeAt(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ok, "resume checkpoint should be recorded")
	s.Require().Equal(newTime.Unix(), resumeAt.Unix())

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(posBefore.LastBonusAccrual, posAfter.LastBonusAccrual,
		"position checkpoint should not be eagerly rewritten")
}

// TestAfterValidatorBonded_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestAfterValidatorBonded_NoPositions() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)
}

func (s *KeeperSuite) TestAfterValidatorBonded_DuplicateKeepsEarliestResumeCheckpoint() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	consAddr := s.recordUnbondingCheckpoint(valAddr)

	firstResume := s.ctx.BlockTime().Add(6 * time.Hour)
	s.recordBondedCheckpointAt(valAddr, consAddr, firstResume)

	secondResume := firstResume.Add(6 * time.Hour)
	s.recordBondedCheckpointAt(valAddr, consAddr, secondResume)

	resumeAt, ok, err := s.keeper.GetValidatorBonusResumeAt(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().Equal(firstResume.Unix(), resumeAt.Unix(), "duplicate bonded hooks should not overwrite first resume checkpoint")
}

// --- AfterValidatorBeginUnbonding hook tests ---

// TestAfterValidatorBeginUnbonding_RecordsPauseCheckpoint verifies that when a
// validator begins unbonding, the hook records pause time lazily without eagerly
// claiming rewards for all positions.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_RecordsPauseCheckpoint() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	_ = s.recordUnbondingCheckpoint(valAddr)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(balBefore.Amount, balAfter.Amount, "hook should not eagerly pay rewards in lazy mode")

	pauseAt, ok, err := s.keeper.GetValidatorBonusPauseAt(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ok, "pause checkpoint should be recorded")
	s.Require().Equal(s.ctx.BlockTime().Unix(), pauseAt.Unix())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	pauseRate, err := s.keeper.ValidatorBonusPauseRate.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedRate := sdkmath.LegacyZeroDec()
	if !val.GetDelegatorShares().IsZero() {
		expectedRate = sdkmath.LegacyNewDecFromInt(val.GetTokens()).Quo(val.GetDelegatorShares())
	}
	s.Require().True(pauseRate.Equal(expectedRate), "pause rate checkpoint should snapshot validator tokens/share at unbonding")
}

// --- AfterValidatorRemoved hook tests ---
func (s *KeeperSuite) TestAfterValidatorRemoved_CleansRewardTrackingState() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	hooks := s.keeper.Hooks()

	err := s.keeper.ValidatorRewardRatio.Set(s.ctx, valAddr, types.ValidatorRewardRatio{
		CumulativeRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.1")),
		},
	})
	s.Require().NoError(err)
	consAddr := s.recordUnbondingCheckpoint(valAddr)
	s.recordBondedCheckpointAt(valAddr, consAddr, s.ctx.BlockTime())

	err = hooks.AfterValidatorRemoved(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	hasRatio, err := s.keeper.ValidatorRewardRatio.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasRatio, "validator reward ratio should be cleaned after validator removal")

	hasPause, err := s.keeper.ValidatorBonusPauseAt.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasPause, "validator bonus pause checkpoint should be cleaned after validator removal")

	hasResume, err := s.keeper.ValidatorBonusResumeAt.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasResume, "validator bonus resume checkpoint should be cleaned after validator removal")

	hasPauseRate, err := s.keeper.ValidatorBonusPauseRate.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasPauseRate, "validator bonus pause rate should be cleaned after validator removal")
}

func (s *KeeperSuite) TestAfterValidatorRemoved_SettlesBeforeClearingCheckpoints() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10_000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	// Build pending pause state without resume.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))
	consAddr := s.recordUnbondingCheckpoint(valAddr)
	removalTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(removalTime)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, bondDenom).Amount

	err := s.keeper.Hooks().AfterValidatorRemoved(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(removalTime, updated.LastBonusAccrual, "validator removal should settle rewards before clearing checkpoints")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, bondDenom).Amount
	s.Require().True(balAfter.GT(balBefore), "owner should receive settled rewards on validator removal")

	hasPause, err := s.keeper.ValidatorBonusPauseAt.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasPause)
}

// --- AfterUnbondingCompleted / AfterRedelegationCompleted hook tests ---

func (s *KeeperSuite) TestAfterUnbondingCompleted_DeletesMapping() {
	hooks := s.keeper.Hooks()

	unbondingId := uint64(42)
	positionId := uint64(1)
	err := s.keeper.UnbondingDelegationMappings.Set(s.ctx, unbondingId, positionId)
	s.Require().NoError(err)

	// Verify mapping exists.
	has, err := s.keeper.UnbondingDelegationMappings.Has(s.ctx, unbondingId)
	s.Require().NoError(err)
	s.Require().True(has)

	// Fire hook with the tier module address (hooks filter by delegator).
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	valAddr := sdk.ValAddress([]byte("validator___________"))
	err = hooks.AfterUnbondingCompleted(s.ctx, poolAddr, valAddr, []uint64{unbondingId})
	s.Require().NoError(err)

	has, err = s.keeper.UnbondingDelegationMappings.Has(s.ctx, unbondingId)
	s.Require().NoError(err)
	s.Require().False(has, "unbonding mapping should be deleted after completion")
}

func (s *KeeperSuite) TestAfterUnbondingCompleted_NoMapping_NoOp() {
	hooks := s.keeper.Hooks()

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	valAddr := sdk.ValAddress([]byte("validator___________"))
	err := hooks.AfterUnbondingCompleted(s.ctx, poolAddr, valAddr, []uint64{999})
	s.Require().NoError(err, "should not error when unbonding ID has no mapping")
}

func (s *KeeperSuite) TestAfterRedelegationCompleted_DeletesMapping() {
	hooks := s.keeper.Hooks()

	unbondingId := uint64(77)
	positionId := uint64(2)
	err := s.keeper.RedelegationMappings.Set(s.ctx, unbondingId, positionId)
	s.Require().NoError(err)

	has, err := s.keeper.RedelegationMappings.Has(s.ctx, unbondingId)
	s.Require().NoError(err)
	s.Require().True(has)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	valSrc := sdk.ValAddress([]byte("validator_src_______"))
	valDst := sdk.ValAddress([]byte("validator_dst_______"))
	err = hooks.AfterRedelegationCompleted(s.ctx, poolAddr, valSrc, valDst, []uint64{unbondingId})
	s.Require().NoError(err)

	has, err = s.keeper.RedelegationMappings.Has(s.ctx, unbondingId)
	s.Require().NoError(err)
	s.Require().False(has, "redelegation mapping should be deleted after completion")
}

func (s *KeeperSuite) TestAfterRedelegationCompleted_NoMapping_NoOp() {
	hooks := s.keeper.Hooks()

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	valSrc := sdk.ValAddress([]byte("validator_src_______"))
	valDst := sdk.ValAddress([]byte("validator_dst_______"))
	err := hooks.AfterRedelegationCompleted(s.ctx, poolAddr, valSrc, valDst, []uint64{888})
	s.Require().NoError(err, "should not error when redelegation ID has no mapping")
}

// --- NoOp callbacks ---

func (s *KeeperSuite) TestHooks_NoOpCallbacks_ReturnNil() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	delAddr := sdk.AccAddress([]byte("noop_delegator_addr"))

	hooks := s.keeper.Hooks()
	s.Require().NoError(hooks.AfterUnbondingInitiated(s.ctx, 1))
	s.Require().NoError(hooks.BeforeValidatorModified(s.ctx, valAddr))
	s.Require().NoError(hooks.AfterValidatorCreated(s.ctx, valAddr))
	s.Require().NoError(hooks.BeforeDelegationCreated(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.BeforeDelegationSharesModified(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.BeforeDelegationRemoved(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.AfterDelegationModified(s.ctx, delAddr, valAddr))
}
