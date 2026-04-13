package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestMsgTriggerExitFromTier_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	resp, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.ExitUnlockAt.IsZero())

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.HasTriggeredExit())
	s.Require().Equal(resp.ExitUnlockAt, pos.ExitUnlockAt)
}

func (s *KeeperSuite) TestMsgTriggerExitFromTier_AlreadyExiting() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionTriggeredExit)
}

func (s *KeeperSuite) TestMsgTriggerExitFromTier_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// TestExitTriggerClearCycles_BonusAccrualCorrectness verifies that
// repeated TriggerExit/ClearPosition cycles do not double-count or under-count
// bonus accrual when cycle durations are identical.
func (s *KeeperSuite) TestExitTriggerClearCycles_BonusAccrualCorrectness() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	cycleDuration := 24 * time.Hour

	balBeforeCycle1 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(cycleDuration))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfterCycle1, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfterCycle1.HasTriggeredExit(), "clear should reset exit state")
	s.Require().True(posAfterCycle1.IsDelegated(), "clear cycle should keep delegated position active")
	s.Require().Equal(s.ctx.BlockTime(), posAfterCycle1.LastBonusAccrual, "clear should checkpoint bonus accrual at current time")

	balAfterCycle1 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	cycle1Payout := balAfterCycle1.Amount.Sub(balBeforeCycle1.Amount)
	s.Require().True(cycle1Payout.IsPositive(), "first cycle should pay positive bonus")

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(cycleDuration))

	_, err = msgServer.ClearPosition(s.ctx, &types.MsgClearPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfterCycle2, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfterCycle2.HasTriggeredExit(), "clear should reset exit state in repeated cycle")
	s.Require().True(posAfterCycle2.IsDelegated(), "repeated clear should keep delegated position active")
	s.Require().Equal(s.ctx.BlockTime(), posAfterCycle2.LastBonusAccrual, "repeated clear should checkpoint to current time")

	balAfterCycle2 := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	cycle2Payout := balAfterCycle2.Amount.Sub(balAfterCycle1.Amount)
	s.Require().True(cycle2Payout.IsPositive(), "second cycle should pay positive bonus")
	s.Require().True(cycle2Payout.Equal(cycle1Payout),
		"equal-duration cycles should pay equal bonus, got cycle1=%s cycle2=%s", cycle1Payout, cycle2Payout)
}
