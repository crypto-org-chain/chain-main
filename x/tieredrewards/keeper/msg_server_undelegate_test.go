package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestMsgTierUndelegate_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create delegated + exit-triggered position

	// Fund the rewards pool so bonus claim doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	s.advancePastExitDuration()
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().False(pos.IsDelegated(), "position should not be delegated after undelegate")
	s.Require().True(pos.DelegatedShares.IsZero(), "delegated shares should be cleared")

	// Verify redelegation unbonding ID was written to UnbondingDelegationMappings, not RedelegationMappings.
	var unbondingFound bool
	err = s.keeper.UnbondingDelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			unbondingFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().True(unbondingFound, "undelegation unbonding ID should be in UnbondingDelegationMappings")

	var redelegationFound bool
	err = s.keeper.RedelegationMappings.Walk(s.ctx, nil, func(_, posId uint64) (bool, error) {
		if posId == pos.Id {
			redelegationFound = true
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)
	s.Require().False(redelegationFound, "undelegation unbonding ID should not be stored in RedelegationMappings")
}

func (s *KeeperSuite) TestMsgTierUndelegate_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// First undelegate succeeds
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Second undelegate should fail with ErrPositionNotDelegated
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierUndelegate_ExitNotTriggered() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

// TestMsgTierUndelegate_ExitDurationNotReached verifies that TierUndelegate is
// rejected when exit is triggered but duration has not elapsed.
func (s *KeeperSuite) TestMsgTierUndelegate_ExitDurationNotReached() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgTierUndelegate_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	s.advancePastExitDuration()
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating verifies that TierUndelegate
// claims pending rewards before undelegating. A subsequent ClaimTierRewards would fail
// (position no longer delegated), but the balance increase confirms rewards were paid.
func (s *KeeperSuite) TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance block so delegation starting period is finalized, then allocate rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.advancePastExitDuration()
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierUndelegate internally claims rewards.
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	balAfterUndelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterUndelegate.Amount.GT(balBefore.Amount), "rewards should be paid during undelegate")
}

// TestMsgTierUndelegate_ReconcilesAmount: after TierUndelegate,
// pos.Amount is reconciled with the actual token return value from the SDK's
// share→token conversion, preventing insolvency on later withdrawal.
func (s *KeeperSuite) TestMsgTierUndelegate_ReconcilesAmount() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash the validator FIRST to create a non-1:1 exchange rate so that
	// share→token conversion actually truncates.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%

	lockAmount := sdkmath.NewInt(10001) // odd number to maximize truncation
	addr := s.fundRandomAddr(bondDenom, lockAmount)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  addr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos := positions[0]
	s.Require().True(pos.IsDelegated())

	// Compute what the SDK will actually return when converting shares→tokens.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedReturn := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	// At the 0.9 exchange rate, round-trip truncation loses 1 token.
	s.Require().Equal(sdkmath.NewInt(10000).String(), expectedReturn.String(),
		"expected return should be 10000 (1 token lost to truncation)")

	s.fundRewardsPool(sdkmath.NewInt(2_000_000), bondDenom)

	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// pos.Amount must equal the actual return value, not the original lockAmount.
	s.Require().Equal(expectedReturn.String(), pos.Amount.String(),
		"pos.Amount must equal actual return value")
	s.Require().Equal(sdkmath.NewInt(10000).String(), pos.Amount.String(),
		"pos.Amount should be exactly 10000 after reconciliation")
}

// TestMsgTierUndelegate_ReconcilesAmountUpward verifies that TierUndelegate
// trusts the staking module's exact return amount even when stored position
// accounting is stale and too low.
func (s *KeeperSuite) TestMsgTierUndelegate_ReconcilesAmountUpward() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]
	s.Require().True(pos.IsDelegated())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	expectedReturn := val.TokensFromShares(pos.DelegatedShares).TruncateInt()
	s.Require().Equal(lockAmount.String(), expectedReturn.String(),
		"test setup expects a 1:1 validator exchange rate")

	// Seed a stale underestimated amount to verify undelegation overwrites it
	// with the staking module's authoritative return amount.
	pos.UpdateAmount(expectedReturn.SubRaw(1))
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(expectedReturn.String(), pos.Amount.String(),
		"pos.Amount must be overwritten with the SDK return amount")

	s.completeStakingUnbonding(valAddr)

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(expectedReturn.String(), resp.Amount.AmountOf(bondDenom).String(),
		"withdrawn amount should equal the SDK return amount")
}

func (s *KeeperSuite) TestMsgTierUndelegate_AfterBondedSlash_Succeeds() {
	lockAmount := sdkmath.NewInt(10_000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(1, 2)) // 1%

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.advancePastExitDuration()

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero(), "undelegation should still succeed after a bonded slash")
}

// TestMsgTierUndelegate_BondedZeroAmount verifies that TierUndelegate succeeds
// on a delegated position with zero amount (100%% bonded slash). The staking
// layer returns zero tokens and the position is cleanly undelegated.
func (s *KeeperSuite) TestMsgTierUndelegate_BondedZeroAmount() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash validator 100% to zero out position amount via hook.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	s.advancePastExitDuration()

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.Amount.IsZero(), "position amount should be zero")
	s.Require().False(pos.IsDelegated(), "position should be undelegated")
}

// TestMsgTierUndelegate_TierCloseOnly_Succeeds verifies that TierUndelegate is
// NOT blocked by CloseOnly — exit-path messages must always succeed.
func (s *KeeperSuite) TestMsgTierUndelegate_TierCloseOnly_Succeeds() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Set tier to CloseOnly.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	// TierUndelegate — should succeed despite CloseOnly.
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero(), "undelegation should succeed on CloseOnly tier")

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated(), "position should be undelegated")
}
