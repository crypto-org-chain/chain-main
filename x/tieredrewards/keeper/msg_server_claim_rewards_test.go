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

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	expectedBonusRewards := s.keeper.CalculateBonusRaw(pos, val, tier, s.ctx.BlockTime())

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

	resp1, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	resp2, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner: addr.String(), Id: 1, Amount: lockAmount, ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)

	claimResp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       addr.String(),
		PositionIds: []uint64{resp1.PositionId, resp2.PositionId},
	})
	s.Require().NoError(err)
	s.Require().Equal([]uint64{resp1.PositionId, resp2.PositionId}, claimResp.PositionIds)
	s.Require().False(claimResp.BaseRewards.IsZero(), "aggregated base rewards should be positive")
	s.Require().False(claimResp.BonusRewards.IsZero(), "aggregated bonus rewards should be positive")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, addr, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount), "owner should receive rewards")
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
