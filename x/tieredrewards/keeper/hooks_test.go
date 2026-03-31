package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// --- BeforeValidatorSlashed hook tests ---

// TestBeforeValidatorSlashed_ClaimsRewardsBeforeSlash verifies that the hook
// claims pending base rewards before applying the slash, so the position's
// BaseRewardsPerShare snapshot is updated and cannot be double-claimed later.
func (s *KeeperSuite) TestBeforeValidatorSlashed_ClaimsRewardsBeforeSlash() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Trigger the slash hook directly with a tiny fraction so positions are affected.
	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2)) // 1% slash
	s.Require().NoError(err)

	// Owner should have received base (and bonus) rewards that were settled before the slash.
	balAfterSlash := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterSlash.Amount.GT(balBefore.Amount), "rewards should have been paid before slash")

	// Calling ClaimTierRewards now should not pay base rewards again (already claimed in hook).
	respClaim, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().True(respClaim.BaseRewards.IsZero(), "base rewards should already be claimed by the slash hook")

	balAfterClaim := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(balAfterSlash.Amount, balAfterClaim.Amount, "second claim should yield nothing extra")
}

// TestBeforeValidatorSlashed_ReducesPositionAmount verifies that the hook reduces
// position.Amount by the expected slash fraction.
func (s *KeeperSuite) TestBeforeValidatorSlashed_ReducesPositionAmount() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	posBefore, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)

	slashFraction := sdkmath.LegacyNewDecWithPrec(5, 2) // 5%
	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.LT(posBefore.Amount), "position amount should decrease after slash")
}

// TestBeforeValidatorSlashed_DoesNotRevertLastBonusAccrual verifies:
// after the hook, the position's LastBonusAccrual in the store reflects the time
// the rewards were settled, not the pre-claim value.
func (s *KeeperSuite) TestBeforeValidatorSlashed_DoesNotRevertLastBonusAccrual() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	claimTime := s.ctx.BlockTime().Add(time.Hour * 24)
	s.ctx = s.ctx.WithBlockTime(claimTime)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	hooks := s.keeper.Hooks()
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)

	// LastBonusAccrual must be updated to claimTime, not left at the initial block time.
	s.Require().Equal(claimTime, pos.LastBonusAccrual, "LastBonusAccrual must be updated after slash hook")
}

// TestBeforeValidatorSlashed_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestBeforeValidatorSlashed_NoPositions() {
	_, valAddr, _ := s.setupTierAndDelegator()
	hooks := s.keeper.Hooks()

	// Should not error when there are no positions.
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)
}

// --- AfterValidatorBonded hook tests ---

// TestAfterValidatorBonded_ResetsLastBonusAccrual verifies that when a validator
// transitions back to bonded, LastBonusAccrual is reset to the current block time
// so bonus does not over-accrue during the unbonding period.
func (s *KeeperSuite) TestAfterValidatorBonded_ResetsLastBonusAccrual() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(1000),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Simulate passage of time.
	newTime := s.ctx.BlockTime().Add(time.Hour * 48)
	s.ctx = s.ctx.WithBlockTime(newTime)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err = hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().Equal(newTime, pos.LastBonusAccrual, "LastBonusAccrual should be reset to current block time")
}

// TestAfterValidatorBonded_NoPositions is a no-op for validators with no tier positions.
func (s *KeeperSuite) TestAfterValidatorBonded_NoPositions() {
	_, valAddr, _ := s.setupTierAndDelegator()

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err := hooks.AfterValidatorBonded(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)
}

// --- AfterValidatorBeginUnbonding hook tests ---

// TestAfterValidatorBeginUnbonding_ClaimsRewards verifies that when a validator
// begins unbonding, base and bonus rewards are claimed for all positions.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_ClaimsRewards() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()
	err = hooks.AfterValidatorBeginUnbonding(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount), "rewards should have been claimed when validator began unbonding")
}

// TestBeforeValidatorSlashed_InsufficientBonusPool verifies the ErrInsufficientBonusPool
// path: when the bonus pool cannot cover the accrued bonus, the hook still completes
// without error, the slash is still applied, and BaseRewardsPerShare is updated
// (base rewards were claimed even though bonus was not paid).
func (s *KeeperSuite) TestBeforeValidatorSlashed_InsufficientBonusPool() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

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
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	hooks := s.keeper.Hooks()
	// 100% slash — must not error even though position.Amount reaches zero.
	err = hooks.BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyOneDec())
	s.Require().NoError(err, "100% slash should not cause an error")

	pos, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)
	s.Require().True(pos.Amount.IsZero(), "position amount should be zero after 100% slash")
}

// TestBeforeValidatorSlashed_MultiplePositions verifies that the slash hook reduces
// all positions delegated to the slashed validator.
func (s *KeeperSuite) TestBeforeValidatorSlashed_MultiplePositions() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	// Create three positions on the same validator.
	for range 3 {
		_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
			Owner:            delAddr.String(),
			Id:               1,
			Amount:           lockAmount,
			ValidatorAddress: valAddr.String(),
		})
		s.Require().NoError(err)
	}

	amountsBefore := make([]sdkmath.Int, 3)
	for i := range 3 {
		pos, err := s.keeper.GetPosition(s.ctx, uint64(i))
		s.Require().NoError(err)
		amountsBefore[i] = pos.Amount
	}

	slashFraction := sdkmath.LegacyNewDecWithPrec(10, 2) // 10%
	hooks := s.keeper.Hooks()
	err := hooks.BeforeValidatorSlashed(s.ctx, valAddr, slashFraction)
	s.Require().NoError(err)

	for i := range 3 {
		pos, err := s.keeper.GetPosition(s.ctx, uint64(i))
		s.Require().NoError(err)
		s.Require().True(pos.Amount.LT(amountsBefore[i]),
			"position %d amount should decrease after slash", i)
	}
}

func (s *KeeperSuite) TestHooks_NoOpCallbacks_ReturnNil() {
	_, valAddr, _ := s.setupTierAndDelegator()
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

func (s *KeeperSuite) TestAfterValidatorRemoved_CleansRewardTrackingState() {
	_, valAddr, _ := s.setupTierAndDelegator()
	consAddr := sdk.ConsAddress(valAddr)
	hooks := s.keeper.Hooks()

	err := s.keeper.ValidatorRewardRatio.Set(s.ctx, valAddr, types.ValidatorRewardRatio{
		CumulativeRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.1")),
		},
	})
	s.Require().NoError(err)
	err = s.keeper.ValidatorRewardsLastWithdrawalBlock.Set(s.ctx, valAddr, uint64(123))
	s.Require().NoError(err)

	err = hooks.AfterValidatorRemoved(s.ctx, consAddr, valAddr)
	s.Require().NoError(err)

	hasRatio, err := s.keeper.ValidatorRewardRatio.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasRatio, "validator reward ratio should be cleaned after validator removal")

	hasWithdrawalMarker, err := s.keeper.ValidatorRewardsLastWithdrawalBlock.Has(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(hasWithdrawalMarker, "validator withdrawal marker should be cleaned after validator removal")
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
