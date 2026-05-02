package keeper_test

import (
	"errors"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestMsgClaimTierRewards_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Undelegate so the position is not delegated
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should be zero when not delegated")
	s.Require().True(resp.BonusRewards.IsZero(), "bonus rewards should be zero when not delegated")
}

func (s *KeeperSuite) TestMsgClaimTierRewards_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       wrongAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgClaimTierRewards_Basic() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	// Lock an amount equal to the genesis delegation so the tier module gets a meaningful share of rewards

	// Advance block and time so distribution period is finalized
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	advanceInTime := time.Hour * 24
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(advanceInTime))

	// Allocate base rewards
	baseRewardsDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, baseRewardsDistributed, bondDenom)

	// Fund the bonus rewards pool
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	tokensPerShare, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	expectedBonusRewards := s.keeper.ComputeSegmentBonus(&pos, tier, pos.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)

	// amount stake is half of whats staked in total, so base rewards are half of the distributed
	expectedBaseRewards := baseRewardsDistributed.Quo(sdkmath.NewInt(2))

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().True(expectedBonusRewards.Equal(resp.BonusRewards.AmountOf(bondDenom)), "bonus rewards should be correct")
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount.Add(expectedBaseRewards.Add(expectedBonusRewards))), "owner should have received rewards matching what's expected")
}

// TestMsgClaimTierRewards_FailsWhenBonusPoolInsufficient verifies that ClaimTierRewards
// returns ErrInsufficientBonusPool when accrued bonus cannot be paid, so the tx rolls
// back and the user can retry later without losing base rewards to a partial claim.
func (s *KeeperSuite) TestMsgClaimTierRewards_FailsWhenBonusPoolInsufficient() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance time and allocate base rewards, but intentionally leave bonus pool empty.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 365))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	// Bonus pool remains at 0 — bonus accrued but pool cannot cover it.
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Use a branched context so a failed message does not persist state (matches DeliverTx rollback).
	cacheCtx, _ := s.ctx.CacheContext()
	resp, err := msgServer.ClaimTierRewards(cacheCtx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().Error(err)
	s.Require().True(errors.Is(err, types.ErrInsufficientBonusPool))
	s.Require().Nil(resp)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.Equal(balBefore.Amount), "failed claim must not transfer rewards")
}

// TestClaimTierRewards_ExitingPosition_EarnRewards verifies that
// bonus rewards accrue until ExitUnlockTime.
func (s *KeeperSuite) TestClaimTierRewards_ExitingPosition_EarnRewards() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit(), "position should be exiting")
	s.Require().True(pos.IsDelegated(), "position should be delegated")
	s.Require().False(pos.LastBonusAccrual.IsZero(), "LastBonusAccrual should be set")

	// Advance time and allocate rewards
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 30)) // 30 days
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Claim rewards — should succeed and pay both base + bonus
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().False(resp.BaseRewards.IsZero() && resp.BonusRewards.IsZero(),
		"exiting-then-delegated position should earn rewards")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount),
		"rewards should have been transferred to owner")
}

// TestMsgClaimTierRewards_EmitsEvent verifies that MsgClaimTierRewards emits
// EventTierRewardsClaimed after a successful claim.
func (s *KeeperSuite) TestMsgClaimTierRewards_EmitsEvent() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	// Isolate events from this specific call.
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err := msgServer.ClaimTierRewards(freshCtx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	found := false
	for _, e := range freshCtx.EventManager().Events() {
		if e.Type == "chainmain.tieredrewards.v1.EventTierRewardsClaimed" {
			found = true
			break
		}
	}
	s.Require().True(found, "EventTierRewardsClaimed should be emitted by ClaimTierRewards")
}

// TestMsgClaimTierRewards_BondedZeroAmount verifies that ClaimTierRewards
// succeeds on a delegated position with zero amount (100% bonded slash).
// Both base and bonus should be zero since TokensFromShares returns 0.
func (s *KeeperSuite) TestMsgClaimTierRewards_BondedZeroAmount() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Advance block so distribution period is finalized.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))

	// Slash validator 100% to zero out position amount via hook.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.Amount.IsZero(), "position amount should be zero after 100%% slash")
	s.Require().True(pos.IsDelegated(), "position should still be delegated")

	// Advance time so bonus would accrue (if amount were non-zero).
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))

	// ClaimRewards — should succeed with zero rewards.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should be zero on zero-amount position")
	s.Require().True(resp.BonusRewards.IsZero(), "bonus rewards should be zero on zero-amount position")
}

// TestMsgClaimTierRewards_MultiplePositions verifies that ClaimTierRewards
// can claim rewards for multiple positions in a single transaction.
func (s *KeeperSuite) TestMsgClaimTierRewards_MultiplePositions() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	// Fund a single address and create two positions.
	addr := s.fundRandomAddr(bondDenom, lockAmount.MulRaw(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Read positions to get their DelegatedShares for expected bonus calculation.
	pos1, err := s.keeper.GetPosition(s.ctx, resp1.PositionId)
	s.Require().NoError(err)
	pos2, err := s.keeper.GetPosition(s.ctx, resp2.PositionId)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))

	baseRewardsDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, baseRewardsDistributed, bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos1.TierId)
	s.Require().NoError(err)

	// Expected bonus: sum of each position's bonus.
	tokensPerShare, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedBonus1 := s.keeper.ComputeSegmentBonus(&pos1, tier, pos1.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)
	expectedBonus2 := s.keeper.ComputeSegmentBonus(&pos2, tier, pos2.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)
	expectedTotalBonus := expectedBonus1.Add(expectedBonus2)

	// Expected base: the module has 2 * lockAmount delegated out of
	// (genesis delegation + 2 * lockAmount) total. Base rewards are proportional.
	// With lockAmount == DefaultPowerReduction (same as genesis delegation),
	// module share = 2/3 of distributed rewards.
	expectedTotalBase := baseRewardsDistributed.MulRaw(2).QuoRaw(3)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)

	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId},
	})
	s.Require().NoError(err)
	s.Require().Equal([]uint64{resp1.PositionId, resp2.PositionId}, claimResp.PositionIds)

	// Verify bonus rewards match expected.
	s.Require().Equal(expectedTotalBonus.String(), claimResp.BonusRewards.AmountOf(bondDenom).String(),
		"aggregated bonus rewards should match sum of per-position calculations")

	// Verify base rewards match expected.
	s.Require().Equal(expectedTotalBase.String(), claimResp.BaseRewards.AmountOf(bondDenom).String(),
		"aggregated base rewards should match proportional share of distributed rewards")

	// Verify balance increased by the sum of base + bonus.
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	expectedTotalRewards := expectedTotalBase.Add(expectedTotalBonus)
	s.Require().Equal(expectedTotalRewards.String(), balAfter.Amount.Sub(balBefore.Amount).String(),
		"balance increase should equal total base + bonus rewards")
}

// TestMsgClaimTierRewards_EmptyPositionIds verifies that an empty position_ids
// array is rejected.
func (s *KeeperSuite) TestMsgClaimTierRewards_EmptyPositionIds() {
	s.setupTier(1)
	addr := sdk.AccAddress([]byte("test_owner__________"))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{},
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "must not be empty")
}

// TestMsgClaimTierRewards_DuplicatePositionIds verifies that duplicate IDs are rejected.
func (s *KeeperSuite) TestMsgClaimTierRewards_DuplicatePositionIds() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id, pos.Id},
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "duplicate")
}

// TestMsgClaimTierRewards_OneWrongOwner verifies that if any position in the
// batch is not owned by the caller, the entire tx fails atomically.
func (s *KeeperSuite) TestMsgClaimTierRewards_OneWrongOwner() {
	pos1 := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos1.Owner)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	// setupNewTierPosition creates a position with a different random address.
	otherPos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	s.Require().NotEqual(pos1.Owner, otherPos.Owner, "positions should have different owners")

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos1.Id, otherPos.Id},
	})
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// TestMsgClaimTierRewards_MultipleValidators verifies batch claiming across
// positions delegated to different validators.
func (s *KeeperSuite) TestMsgClaimTierRewards_MultipleValidators() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr0 := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	// Create a second validator (self-delegation = 1_000_000).
	valAddr1, _ := s.createSecondValidator()

	// Fund a single address and create two positions on different validators.
	addr := s.fundRandomAddr(bondDenom, lockAmount.MulRaw(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr0, sdkmath.LegacyZeroDec())
	s.setValidatorCommission(valAddr1, sdkmath.LegacyZeroDec())

	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr0.String(),
	})
	s.Require().NoError(err)

	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr1.String(),
	})
	s.Require().NoError(err)

	pos1, err := s.keeper.GetPosition(s.ctx, resp1.PositionId)
	s.Require().NoError(err)
	pos2, err := s.keeper.GetPosition(s.ctx, resp2.PositionId)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))

	baseDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr0, baseDistributed, bondDenom)
	s.allocateRewardsToValidator(valAddr1, baseDistributed, bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	val0, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr0)
	s.Require().NoError(err)
	val1, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr1)
	s.Require().NoError(err)
	tier, err := s.keeper.Tiers.Get(s.ctx, pos1.TierId)
	s.Require().NoError(err)

	// Expected bonus: sum from each validator.
	tokensPerShare0 := val0.TokensFromShares(sdkmath.LegacyOneDec())
	tokensPerShare1 := val1.TokensFromShares(sdkmath.LegacyOneDec())
	expectedBonus1 := s.keeper.ComputeSegmentBonus(&pos1, tier, pos1.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare0)
	expectedBonus2 := s.keeper.ComputeSegmentBonus(&pos2, tier, pos2.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare1)
	expectedTotalBonus := expectedBonus1.Add(expectedBonus2)

	// Expected base: on val0, module has lockAmount out of (genesis + lockAmount) = 1/2.
	// On val1, module has lockAmount out of (self-del + lockAmount) = 1/2.
	expectedBase0 := baseDistributed.QuoRaw(2)
	expectedBase1 := baseDistributed.QuoRaw(2)
	expectedTotalBase := expectedBase0.Add(expectedBase1)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)

	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId},
	})
	s.Require().NoError(err)

	s.Require().Equal(expectedTotalBonus.String(), claimResp.BonusRewards.AmountOf(bondDenom).String(),
		"aggregated bonus rewards should match sum of per-position calculations")
	s.Require().Equal(expectedTotalBase.String(), claimResp.BaseRewards.AmountOf(bondDenom).String(),
		"aggregated base rewards should match proportional share across validators")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	expectedTotal := expectedTotalBase.Add(expectedTotalBonus)
	s.Require().Equal(expectedTotal.String(), balAfter.Amount.Sub(balBefore.Amount).String(),
		"balance increase should equal total base + bonus rewards")
}

// TestMsgClaimTierRewards_MultipleTiers verifies batch claiming across
// positions in different tiers on the same validator.
func (s *KeeperSuite) TestMsgClaimTierRewards_MultipleTiers() {
	s.setupTier(1)
	s.setupTier(2)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	addr := s.fundRandomAddr(bondDenom, lockAmount.MulRaw(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 2, Amount: lockAmount, ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	pos1, err := s.keeper.GetPosition(s.ctx, resp1.PositionId)
	s.Require().NoError(err)
	pos2, err := s.keeper.GetPosition(s.ctx, resp2.PositionId)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))

	baseDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, baseDistributed, bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	tier1, err := s.keeper.Tiers.Get(s.ctx, uint32(1))
	s.Require().NoError(err)
	tier2, err := s.keeper.Tiers.Get(s.ctx, uint32(2))
	s.Require().NoError(err)

	// Expected bonus: each position uses its own tier's BonusApy.
	tokensPerShare, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedBonus1 := s.keeper.ComputeSegmentBonus(&pos1, tier1, pos1.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)
	expectedBonus2 := s.keeper.ComputeSegmentBonus(&pos2, tier2, pos2.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)
	expectedTotalBonus := expectedBonus1.Add(expectedBonus2)

	// Expected base: module has 2 * lockAmount out of (genesis + 2 * lockAmount) = 2/3.
	expectedTotalBase := baseDistributed.MulRaw(2).QuoRaw(3)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)

	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId},
	})
	s.Require().NoError(err)

	s.Require().Equal(expectedTotalBonus.String(), claimResp.BonusRewards.AmountOf(bondDenom).String(),
		"aggregated bonus rewards should match sum of per-position calculations across tiers")
	s.Require().Equal(expectedTotalBase.String(), claimResp.BaseRewards.AmountOf(bondDenom).String(),
		"aggregated base rewards should match proportional share of distributed rewards")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	expectedTotal := expectedTotalBase.Add(expectedTotalBonus)
	s.Require().Equal(expectedTotal.String(), balAfter.Amount.Sub(balBefore.Amount).String(),
		"balance increase should equal total base + bonus rewards")
}

// TestMsgClaimTierRewards_MultipleValidatorsAndTiers verifies batch claiming
// across positions on different validators AND different tiers simultaneously.
// This exercises the nested grouping logic (validator → tier) with all
// combinations: val0/tier1, val0/tier2, val1/tier1.
func (s *KeeperSuite) TestMsgClaimTierRewards_MultipleValidatorsAndTiers() {
	s.setupTier(1)
	s.setupTier(2)
	vals, bondDenom := s.getStakingData()
	valAddr0 := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	valAddr1, _ := s.createSecondValidator()

	addr := s.fundRandomAddr(bondDenom, lockAmount.MulRaw(3))
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr0, sdkmath.LegacyZeroDec())
	s.setValidatorCommission(valAddr1, sdkmath.LegacyZeroDec())

	// Position 1: val0, tier1
	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr0.String(),
	})
	s.Require().NoError(err)

	// Position 2: val0, tier2
	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 2, Amount: lockAmount, ValidatorAddress: valAddr0.String(),
	})
	s.Require().NoError(err)

	// Position 3: val1, tier1
	resp3, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr1.String(),
	})
	s.Require().NoError(err)

	pos1, err := s.keeper.GetPosition(s.ctx, resp1.PositionId)
	s.Require().NoError(err)
	pos2, err := s.keeper.GetPosition(s.ctx, resp2.PositionId)
	s.Require().NoError(err)
	pos3, err := s.keeper.GetPosition(s.ctx, resp3.PositionId)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))

	baseDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr0, baseDistributed, bondDenom)
	s.allocateRewardsToValidator(valAddr1, baseDistributed, bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	val0, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr0)
	s.Require().NoError(err)
	val1, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr1)
	s.Require().NoError(err)
	tier1, err := s.keeper.Tiers.Get(s.ctx, uint32(1))
	s.Require().NoError(err)
	tier2, err := s.keeper.Tiers.Get(s.ctx, uint32(2))
	s.Require().NoError(err)

	// Expected bonus: each position uses its own validator + tier.
	tokensPerShare0 := val0.TokensFromShares(sdkmath.LegacyOneDec())
	tokensPerShare1 := val1.TokensFromShares(sdkmath.LegacyOneDec())
	expectedBonus1 := s.keeper.ComputeSegmentBonus(&pos1, tier1, pos1.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare0)
	expectedBonus2 := s.keeper.ComputeSegmentBonus(&pos2, tier2, pos2.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare0)
	expectedBonus3 := s.keeper.ComputeSegmentBonus(&pos3, tier1, pos3.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare1)
	expectedTotalBonus := expectedBonus1.Add(expectedBonus2).Add(expectedBonus3)

	// Expected base: val0 has 2 positions (2/3 of total), val1 has 1 position (1/2 of total).
	expectedBaseVal0 := baseDistributed.MulRaw(2).QuoRaw(3)
	expectedBaseVal1 := baseDistributed.QuoRaw(2)
	expectedTotalBase := expectedBaseVal0.Add(expectedBaseVal1)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)

	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId, resp3.PositionId},
	})
	s.Require().NoError(err)
	s.Require().Equal(
		[]uint64{resp1.PositionId, resp2.PositionId, resp3.PositionId},
		claimResp.PositionIds,
	)

	s.Require().Equal(expectedTotalBonus.String(), claimResp.BonusRewards.AmountOf(bondDenom).String(),
		"aggregated bonus should match sum across validators and tiers")
	s.Require().Equal(expectedTotalBase.String(), claimResp.BaseRewards.AmountOf(bondDenom).String(),
		"aggregated base should match proportional shares across validators")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	expectedTotal := expectedTotalBase.Add(expectedTotalBonus)
	s.Require().Equal(expectedTotal.String(), balAfter.Amount.Sub(balBefore.Amount).String(),
		"balance increase should equal total base + bonus rewards")
}

// TestMsgClaimTierRewards_MixDelegatedAndUndelegated verifies that a batch
// with some delegated and some undelegated positions claims only for delegated
// ones and succeeds overall.
func (s *KeeperSuite) TestMsgClaimTierRewards_MixDelegatedAndUndelegated() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	addr := s.fundRandomAddr(bondDenom, lockAmount.MulRaw(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Position 1: delegated
	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	// Position 2: delegated (will be undelegated)
	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	s.advancePastExitDuration()

	// Undelegate position 2
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: addr.String(), PositionId: resp2.PositionId,
	})
	s.Require().NoError(err)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

	// Batch claim both: pos1 (delegated) + pos2 (undelegated)
	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId},
	})
	s.Require().NoError(err)
	s.Require().Equal([]uint64{resp1.PositionId, resp2.PositionId}, claimResp.PositionIds)
	s.Require().False(claimResp.BaseRewards.IsZero(),
		"should have base rewards from delegated position")
}

// TestMsgClaimTierRewards_AllUndelegated verifies that a batch claim where
// ALL positions are undelegated succeeds with zero rewards.
func (s *KeeperSuite) TestMsgClaimTierRewards_AllUndelegated() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	addr := s.fundRandomAddr(bondDenom, lockAmount.MulRaw(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	s.advancePastExitDuration()

	// Undelegate both positions.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: addr.String(), PositionId: resp1.PositionId,
	})
	s.Require().NoError(err)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: addr.String(), PositionId: resp2.PositionId,
	})
	s.Require().NoError(err)

	// Batch claim on two undelegated positions — should succeed with zero rewards.
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId},
	})
	s.Require().NoError(err)
	s.Require().Equal([]uint64{resp1.PositionId, resp2.PositionId}, claimResp.PositionIds)
	s.Require().True(claimResp.BaseRewards.IsZero(), "base rewards should be zero for all-undelegated batch")
	s.Require().True(claimResp.BonusRewards.IsZero(), "bonus rewards should be zero for all-undelegated batch")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	s.Require().Equal(balBefore.Amount.String(), balAfter.Amount.String(),
		"balance should not change for all-undelegated batch")
}

// TestMsgClaimTierRewards_OwnerReceivesRewards confirms
// that base rewards and bonus rewards both land at the owner's account.
func (s *KeeperSuite) TestMsgClaimTierRewards_OwnerReceivesRewards() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	posDelAddr := types.GetDelegatorAddress(pos.Id)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Pre-claim: posDelAddr should already be empty (delegation consumes the funds).
	s.Require().True(s.app.BankKeeper.GetBalance(s.ctx, posDelAddr, bondDenom).Amount.IsZero(),
		"position's delegation address must hold no bondDenom pre-claim")

	// Let time pass so bonus accrues, then accrue base rewards on the validator.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000), bondDenom)

	ownerBalBefore := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, bondDenom)

	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       pos.Owner,
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)

	// Sanity: both reward streams produced something — otherwise the test
	// isn't actually exercising both routing paths.
	s.Require().True(resp.BaseRewards.AmountOf(bondDenom).IsPositive(), "base rewards should be positive")
	s.Require().True(resp.BonusRewards.AmountOf(bondDenom).IsPositive(), "bonus rewards should be positive")

	expectedTotal := resp.BaseRewards.AmountOf(bondDenom).Add(resp.BonusRewards.AmountOf(bondDenom))

	ownerBalAfter := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, bondDenom)
	s.Require().Equal(ownerBalBefore.Amount.Add(expectedTotal).String(), ownerBalAfter.Amount.String(),
		"owner balance must increase by base + bonus")
}
