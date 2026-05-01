package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMsgTierDelegate_Basic() {
	// Create delegated position
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redelegation slash zeroing the position
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Add funds back
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     lockAmt,
	})
	s.Require().NoError(err)

	// Delegate position
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().False(pos.LastBonusAccrual.IsZero(), "LastBonusAccrual should be set")
	s.Require().Equal(uint64(0), pos.LastEventSeq, "LastEventSeq should be 0 for fresh validator")
}

func (s *KeeperSuite) TestMsgTierDelegate_AlreadyDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create position with delegation

	// Try to delegate again
	_, err := msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAlreadyDelegated)
}

// TestMsgTierDelegate_AmountZero verifies that TierDelegate is rejected on a
// zero-amount position
func (s *KeeperSuite) TestMsgTierDelegate_AmountZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash clearing delegation and zeros amount
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)
}

// TestMsgTierDelegate_AmountZero_TriggeredExit verifies that TierDelegate is rejected on a
// zero-amount position with exit triggered.
func (s *KeeperSuite) TestMsgTierDelegate_AmountZero_ExitInProgress() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Simulate redelegation slash clearing delegation and zeros amount
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrPositionAmountZero)
}

// TestMsgTierDelegate_ExitInProgress verifies that TierDelegate succeeds when
// exit is triggered but not yet elapsed on an undelegated position.
func (s *KeeperSuite) TestMsgTierDelegate_ExitInProgress() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redelegation slash zeroing out the position
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Add funds back
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	// Trigger exit
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))

	// Delegate while exit is in progress
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().True(pos.HasTriggeredExit())
	s.Require().False(pos.CompletedExitLockDuration(s.ctx.BlockTime()))
}

// TestMsgTierDelegate_ExitElapsed verifies that TierDelegate is rejected when
// exit has fully elapsed.
func (s *KeeperSuite) TestMsgTierDelegate_ExitElapsed() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance past exit duration, then undelegate + complete unbonding.
	s.advancePastExitDuration()
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	// Delegate after exit elapsed — should be rejected.
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationElapsed)
}

func (s *KeeperSuite) TestMsgTierDelegate_WrongOwner() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redeleg slash to get undelegated position without exit.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgTierDelegate_ValidatorIndexUpdated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redeleg slash to get undelegated position.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1000),
	})
	s.Require().NoError(err)

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	// After delegation, position should appear in validator index
	posIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(posIds, 1)
	s.Require().Equal(uint64(0), posIds[0])
}

// TestMsgTierDelegate_TierCloseOnly verifies that TierDelegate is rejected
// when the tier is set to CloseOnly.
func (s *KeeperSuite) TestMsgTierDelegate_TierCloseOnly() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Simulate redelegation slash: clear delegation and zero amount.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	// Add funds back so position has non-zero amount for delegation.
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     lockAmt,
	})
	s.Require().NoError(err)

	// Set tier to close only.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Validator:  valAddr.String(),
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}
