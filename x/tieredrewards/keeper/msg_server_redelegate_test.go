package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"time"
)

func (s *KeeperSuite) TestMsgTierRedelegate_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create a second validator to redelegate to
	dstValAddr, _ := s.createSecondValidator()
	// Create delegated position

	resp, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(dstValAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())

	// Verify redelegation unbonding ID was written to RedelegationMappings, not UnbondingDelegationMappings.
	var redelegationFound bool
	err = s.keeper.RedelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			redelegationFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(redelegationFound, "redelegation unbonding ID should be stored in RedelegationMappings")

	var unbondingFound bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			unbondingFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().False(unbondingFound, "redelegation unbonding ID should NOT be in UnbondingDelegationMappings")
}

func (s *KeeperSuite) TestMsgTierRedelegate_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Undelegate first so position is not delegated
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierRedelegate_SameValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create delegated position

	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrRedelegationToSameValidator)
}

// TestMsgTierRedelegate_AmountZero verifies that TierRedelegate is rejected on a
// zero-amount bonded position (slash from bonded validator)
func (s *KeeperSuite) TestMsgTierRedelegate_AmountZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate slash by zeroing amount
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	dstValidator := sdk.ValAddress([]byte("dst_validator________"))
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValidator.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)
}

// TestMsgTierRedelegate_ExitInProgress verifies that TierRedelegate succeeds
// when exit is in progress.
func (s *KeeperSuite) TestMsgTierRedelegate_ExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Exit is triggered but NOT elapsed.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(dstValAddr.String(), pos.Validator)
	s.Require().True(pos.HasTriggeredExit())
}

// TestMsgTierRedelegate_ExitElapsed verifies that TierRedelegate is rejected
// when exit has fully elapsed — user must ClearPosition first.
func (s *KeeperSuite) TestMsgTierRedelegate_ExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.advancePastExitDuration()
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Exit has elapsed, position still delegated.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationElapsed)
}

func (s *KeeperSuite) TestMsgTierRedelegate_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        wrongAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgTierRedelegate_UpdatesValidatorIndex() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Position should be in source validator index
	srcIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(srcIds, 1)

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	// Source validator index should be empty
	srcIds, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Empty(srcIds)

	// Destination validator index should have the position
	dstIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	s.Require().Len(dstIds, 1)
	s.Require().Equal(uint64(0), dstIds[0])
}

// TestMsgTierRedelegate_ClaimsRewardsBeforeRedelegating verifies that TierRedelegate
// claims pending rewards before performing the redelegation. A subsequent ClaimTierRewards
// call (with no new rewards allocated) should yield zero base rewards.
func (s *KeeperSuite) TestMsgTierRedelegate_ClaimsRewardsBeforeRedelegating() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	dstValAddr, _ := s.createSecondValidator()

	// Advance time and allocate rewards so there are pending base rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierRedelegate internally claims rewards.
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	balAfterRedelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterRedelegate.Amount.GT(balBefore.Amount), "rewards should be paid during redelegate")

	// No new rewards allocated — subsequent ClaimTierRewards on dst validator should yield zero base.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should already be claimed during redelegate")
}

// TestMsgTierRedelegate_ReconcilesAmount: after TierRedelegate at non-1:1
// exchange rate, pos.Amount matches the destination validator's share-backed
// token value.
func (s *KeeperSuite) TestMsgTierRedelegate_ReconcilesAmount() {
	lockAmount := sdkmath.NewInt(10001)
	_, bondDenom := s.getStakingData()
	pos := s.setupNewTierPosition(lockAmount, false)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	dstValAddr, _ := s.createSecondValidator()

	// Slash source validator to create non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	posId := positions[0].Id

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)

	// Redelegate to destination validator — this should reconcile pos.Amount.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        addr.String(),
		PositionId:   posId,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, posId)
	s.Require().NoError(err)

	// Verify pos.Amount matches destination validator's share value.
	dstVal, err := s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	actualTokenValue := dstVal.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from destination validator's shares after TierRedelegate")
}

// TestMsgTierRedelegate_TierCloseOnly verifies that TierRedelegate is rejected
// when the tier is set to CloseOnly.
func (s *KeeperSuite) TestMsgTierRedelegate_TierCloseOnly() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	dstValAddr, _ := s.createSecondValidator()

	// Set tier to close only.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        delAddr.String(),
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}