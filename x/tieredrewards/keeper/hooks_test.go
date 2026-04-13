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

// TestBeforeValidatorSlashed_ReducesPositionAmount verifies that the hook reduces
// position.Amount by the expected slash fraction.
func (s *KeeperSuite) TestBeforeValidatorSlashed_ReducesPositionAmount() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// 1/3 slash intentionally produces a fractional post-slash token amount
	// for common integer token values, so we can assert truncation behavior.
	slashFraction := sdkmath.LegacyMustNewDecFromStr("0.333333333333333333")
	expectedDec := valBefore.TokensFromShares(posBefore.DelegatedShares).Mul(sdkmath.LegacyOneDec().Sub(slashFraction))
	expectedAmount := expectedDec.TruncateInt()
	s.Require().False(expectedDec.IsInteger(), "test assumption: expected post-slash amount should include fractional dust")

	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.Equal(expectedAmount),
		"position amount should match truncated post-slash expectation; got %s want %s",
		posAfter.Amount, expectedAmount,
	)
	s.Require().True(posAfter.Amount.LT(posBefore.Amount), "position amount should decrease after slash")
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
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	hooks := s.keeper.Hooks()

	// Should not error when there are no positions.
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)
}

// TestBeforeValidatorSlashed_InsufficientBonusPool verifies the ErrInsufficientBonusPool
// path: when the bonus pool cannot cover the accrued bonus, the hook still completes
// without error, the slash is still applied, and BaseRewardsPerShare is updated
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

	// Slash must still be applied.
	s.Require().True(posAfter.Amount.LT(posBefore.Amount), "position amount should decrease even when bonus pool is empty")

	// Base rewards must still be claimed (BaseRewardsPerShare updated).
	s.Require().NotEqual(oldRatio, posAfter.BaseRewardsPerShare,
		"BaseRewardsPerShare must be updated even when bonus pool is empty")
	s.Require().Equal(claimTime, posAfter.LastBonusAccrual,
		"LastBonusAccrual must still advance when bonus pool is empty")
}

// TestBeforeValidatorSlashed_FullSlash_DoesNotHaltChain verifies that a 100% slash
// reduces position.Amount to zero without triggering a validation error.
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
	s.Require().True(posAfter.Amount.IsZero(), "position amount should be zero after 100% slash")
}

// TestBeforeValidatorSlashed_MultiplePositions verifies that the slash hook reduces
// all positions delegated to the slashed validator.
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
		s.Require().True(posAfter.Amount.LT(amountsBefore[i]),
			"position %d amount should decrease after slash", p.Id)
	}
}

// TestBeforeValidatorSlashed_MultiplePositions_InsufficientBonusPool verifies
// that the hook can reuse the in-memory positions updated during reward
// settlement, without reloading from store, while still advancing checkpoints
// and applying the slash to every position.
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
	s.Require().True(pos0After.Amount.LT(pos0Before.Amount),
		"first position amount should still be slashed")
	s.Require().True(pos1After.Amount.LT(pos1Before.Amount),
		"second position amount should still be slashed")
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

// --- AfterValidatorBonded hook tests ---

// TestAfterValidatorBonded_ResetsLastBonusAccrual verifies that when a validator
// transitions back to bonded, LastBonusAccrual is reset to the current block time
// so bonus does not over-accrue during the unbonding period.
func (s *KeeperSuite) TestAfterValidatorBonded_ResetsLastBonusAccrual() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate passage of time.
	newTime := s.ctx.BlockTime().Add(time.Hour * 48)
	s.ctx = s.ctx.WithBlockTime(newTime)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(newTime, posAfter.LastBonusAccrual, "LastBonusAccrual should be reset to current block time")
}

// TestAfterValidatorBonded_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestAfterValidatorBonded_NoPositions() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err = hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)
}

// --- AfterValidatorBeginUnbonding hook tests ---

// TestAfterValidatorBeginUnbonding_ClaimsRewards verifies that when a validator
// begins unbonding, base and bonus rewards are claimed for all positions.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_ClaimsRewards() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBeginUnbonding(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount), "rewards should have been claimed when validator began unbonding")
}

// --- AfterValidatorRemoved hook tests ---
func (s *KeeperSuite) TestAfterValidatorRemoved_CleansRewardTrackingState() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()

	err = s.keeper.ValidatorRewardRatio.Set(s.ctx, valAddr, types.ValidatorRewardRatio{
		CumulativeRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.1")),
		},
	})
	s.Require().NoError(err)

	err = hooks.AfterValidatorRemoved(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	hasRatio, err := s.keeper.ValidatorRewardRatio.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasRatio, "validator reward ratio should be cleaned after validator removal")
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
	valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	consAddr := sdk.ConsAddress(valAddr)
	delAddr := sdk.AccAddress([]byte("noop_delegator_addr"))

	hooks := s.keeper.Hooks()
	s.Require().NoError(hooks.BeforeValidatorModified(s.ctx, valAddr))
	s.Require().NoError(hooks.AfterValidatorRemoved(s.ctx, consAddr, valAddr))
	s.Require().NoError(hooks.AfterValidatorCreated(s.ctx, valAddr))
	s.Require().NoError(hooks.BeforeDelegationCreated(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.BeforeDelegationSharesModified(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.BeforeDelegationRemoved(s.ctx, delAddr, valAddr))
	s.Require().NoError(hooks.AfterDelegationModified(s.ctx, delAddr, valAddr))
}
