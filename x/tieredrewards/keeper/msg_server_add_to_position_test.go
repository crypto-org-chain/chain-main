package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	"time"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Undelegated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Simulate redelegation slash clearing delegation and zeroing amount.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.ZeroInt())
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	_, bondDenom := s.getStakingData()
	addPosAmt := sdkmath.NewInt(500)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addPosAmt,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(addPosAmt.Equal(pos.Amount), "amount should be 500 (added to zero)")
	s.Require().False(pos.IsDelegated(), "position should remain undelegated")
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Delegated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	posBefore, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	addPosAmt := sdkmath.NewInt(500)
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(sdkmath.NewInt(1500).Equal(posAfter.Amount), "amount should be 1500")
	s.Require().True(posAfter.DelegatedShares.GT(posBefore.DelegatedShares), "shares should increase")
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Exiting() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrPositionTriggeredExit)
}

func (s *KeeperSuite) TestMsgAddToTierPosition_TierCloseOnly() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set tier to close only
	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err := msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().ErrorIs(err, types.ErrTierIsCloseOnly)
}

func (s *KeeperSuite) TestMsgAddToTierPosition_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	_, err := msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(500),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// TestMsgAddToTierPosition_NeverDoubleClaimsRewards verifies that AddToTierPosition
// re-fetches the position after ClaimRewardsForPositions, so the updated
// BaseRewardsPerShare snapshot is not overwritten. Calling ClaimTierRewards a second
// time should yield zero base rewards if no new rewards have been allocated.
func (s *KeeperSuite) TestMsgAddToTierPosition_NeverDoubleClaimsRewards() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Allocate some base rewards before adding to position.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	addPosAmt := sdkmath.NewInt(500)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)

	// AddToTierPosition internally claims rewards before delegating new tokens.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addPosAmt,
	})
	s.Require().NoError(err)

	balAfterAdd := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// No new rewards have been allocated — a second claim should yield zero.
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	balAfterClaim := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Balance should not change: the first claim was already settled in AddToTierPosition.
	s.Require().Equal(balAfterAdd.Amount, balAfterClaim.Amount, "no additional rewards should be paid on second claim")
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should be zero on second claim")
	s.Require().True(resp.BonusRewards.IsZero(), "bonus rewards should also be zero on second claim: already settled in AddToTierPosition")
}

// TestMsgAddToTierPosition_EmitsBonusRewardsClaimedSideEffect verifies that
// AddToTierPosition on a delegated position emits EventBonusRewardsClaimed as
// a side effect of the implicit reward claim before adding tokens.
// This covers the skipped integration test test_add_to_position_reward_side_effect.
func (s *KeeperSuite) TestMsgAddToTierPosition_EmitsBonusRewardsClaimedSideEffect() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	_, bondDenom := s.getStakingData()
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Advance time so bonus accrues (LastBonusAccrual is initialized at position creation by WithDelegation).
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	addToPosAmt := sdkmath.NewInt(1000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addToPosAmt)))
	s.Require().NoError(err)
	freshCtx := s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.AddToTierPosition(freshCtx, &types.MsgAddToTierPosition{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     addToPosAmt,
	})
	s.Require().NoError(err)

	found := false
	for _, e := range freshCtx.EventManager().Events() {
		if e.Type == "chainmain.tieredrewards.v1.EventBonusRewardsClaimed" {
			found = true
			break
		}
	}
	s.Require().True(found, "EventBonusRewardsClaimed should be emitted as side effect of AddToTierPosition on a delegated position")
}

// TestMsgAddToTierPosition_ReconcilesAmountWithShares: after
// AddToTierPosition on a delegated position, pos.Amount matches the actual
// token value from total shares, not the arithmetic sum of deposits.
func (s *KeeperSuite) TestMsgAddToTierPosition_ReconcilesAmountWithShares() {
	lockAmount := sdkmath.NewInt(10001)
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Slash the validator to create a non-1:1 exchange rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2)) // 10%
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos = positions[0]

	addAmount := sdkmath.NewInt(5001)
	_, bondDenom := s.getStakingData()
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      addr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// pos.Amount must equal what the validator says the total shares are worth.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"pos.Amount must equal actual token value from shares")

	// The reconciled amount must be strictly less than the arithmetic sum
	// because the non-1:1 exchange rate causes truncation loss.
	arithmeticSum := lockAmount.Add(addAmount) // 15002
	s.Require().NotEqual(arithmeticSum.String(), pos.Amount.String(),
		"pos.Amount must differ from naive arithmetic sum due to truncation")
}

// TestMsgAddToTierPosition_MultipleCalls_NoDivergence verifies that repeated
// AddToTierPosition calls don't compound rounding divergence. After N calls,
// pos.Amount still matches TokensFromShares(totalShares).
func (s *KeeperSuite) TestMsgAddToTierPosition_MultipleCalls_NoDivergence() {
	initialAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(initialAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Slash to get non-1:1 rate.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))
	addPerCall := sdkmath.NewInt(1001) // odd to maximize truncation
	numAdds := 5
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	posId := positions[0].Id

	_, bondDenom := s.getStakingData()
	totalNeeded := addPerCall.MulRaw(int64(numAdds))
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr, sdk.NewCoins(sdk.NewCoin(bondDenom, totalNeeded)))
	s.Require().NoError(err)
	for i := 0; i < numAdds; i++ {
		_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
			Owner:      addr.String(),
			PositionId: posId,
			Amount:     addPerCall,
		})
		s.Require().NoError(err)
	}

	pos, err = s.keeper.GetPosition(s.ctx, posId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	actualTokenValue := val.TokensFromShares(pos.DelegatedShares).TruncateInt()

	s.Require().Equal(actualTokenValue.String(), pos.Amount.String(),
		"after %d additions, pos.Amount must equal actual token value from shares", numAdds)

	// The reconciled amount must be strictly less than the arithmetic sum.
	// Without the fix, pos.Amount would equal arithmeticSum (15005), diverging
	// from the actual share-backed value by up to numAdds tokens.
	arithmeticSum := initialAmount.Add(addPerCall.MulRaw(int64(numAdds)))
	s.Require().NotEqual(arithmeticSum.String(), pos.Amount.String(),
		"pos.Amount must differ from naive arithmetic sum due to truncation")
}

// TestMsgAddToTierPosition_DelegatedToSlashedValidator verifies that AddToTier
// fails when the position is delegated to a validator that has been 100%%
// slashed, creating an invalid share/token exchange rate.
func (s *KeeperSuite) TestMsgAddToTierPosition_DelegatedToSlashedValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash validator 100% — burns most tokens but may leave dust due to
	// power truncation. The BeforeValidatorSlashed hook settles rewards and
	// sets position amount to zero.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	// Force validator tokens to exactly zero so InvalidExRate() returns true.
	// The 100% slash through staking may leave dust from power truncation.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	val.Tokens = sdkmath.ZeroInt()
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, val))

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated(), "position should still be delegated")

	// Fund owner and attempt AddToTier — delegate should fail due to invalid exchange rate.
	addAmount := sdkmath.NewInt(500)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().Error(err, "AddToTier should fail on validator with invalid exchange rate")
	s.Require().ErrorIs(err, stakingtypes.ErrDelegatorShareExRateInvalid)
}	