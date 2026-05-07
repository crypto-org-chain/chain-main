package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Undelegated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Simulate redelegation slash clearing delegation and zeroing amount.
	pos = s.slashRedelegationCompletely(pos)

	_, bondDenom := s.getStakingData()
	addPosAmt := sdkmath.NewInt(500)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addPosAmt)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addPosAmt,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(addPosAmt.Equal(s.getPositionAmount(pos)), "amount should be 500 (added to zero)")
	s.Require().False(pos.IsDelegated(), "position should remain undelegated")
}

func (s *KeeperSuite) TestMsgAddToTierPosition_Basic_Delegated() {
	lockAmt := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmt, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	posBefore, err := s.keeper.GetPositionState(s.ctx, pos.Id)
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

	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	positionAmount, err := s.keeper.GetPositionAmount(s.ctx, posAfter)
	s.Require().NoError(err)
	s.Require().True(positionAmount.GTE(sdkmath.NewInt(1500)), "token value should be at least 1500")
	s.Require().True(posAfter.Delegation.Shares.GT(posBefore.Delegation.Shares), "shares should increase")
	s.Require().Equal(uint64(0), posAfter.LastEventSeq, "LastEventSeq should be 0 for fresh validator")

	valCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount)
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
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
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

// TestMsgAddToTierPosition_NeverDoubleClaimsRewards verifies that calling ClaimTierRewards after adding to a tier position
// should yield zero base rewards on the same block.
func (s *KeeperSuite) TestMsgAddToTierPosition_NeverDoubleClaimsRewards() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
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
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	balAfterClaim := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Balance should not change: the first claim was already settled in AddToTierPosition.
	s.Require().Equal(balAfterAdd.Amount, balAfterClaim.Amount, "no additional rewards should be paid on second claim")
	s.Require().True(resp.BaseRewards.IsZero(), "base rewards should be zero on second claim")
	s.Require().True(resp.BonusRewards.IsZero(), "bonus rewards should also be zero on second claim: already settled in AddToTierPosition")
}

// TestMsgAddToTierPosition_EmitsBonusRewardsClaimedSideEffect verifies that
// AddToTierPosition on a delegated position emits EventBaseReawrdsClaimed and EventBonusRewardsClaimed after a successful claim.
func (s *KeeperSuite) TestMsgAddToTierPosition_EmitsRewardsClaimedEvents() {
	lockAmt := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmt, false)
	_, bondDenom := s.getStakingData()
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance block so the delegation's starting period is finalized,
	// then allocate base rewards so EventBaseRewardsClaimed fires.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

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

	foundBase := false
	foundBonus := false
	for _, e := range freshCtx.EventManager().Events() {
		switch e.Type {
		case "chainmain.tieredrewards.v1.EventBonusRewardsClaimed":
			foundBonus = true
		case "chainmain.tieredrewards.v1.EventBaseRewardsClaimed":
			foundBase = true
		}
		if foundBase && foundBonus {
			break
		}
	}
	s.Require().True(foundBase, "EventBaseRewardsClaimed should be emitted")
	s.Require().True(foundBonus, "EventBonusRewardsClaimed should be emitted")
}

// TestMsgAddToTierPosition_DelegatedToSlashedValidator verifies that AddToTier
// fails after a 100% slash when the validator ends up with an invalid
// (zero token) exchange rate. The claim inside AddToTierPosition should
// still succeed — only the subsequent delegate call should fail.
func (s *KeeperSuite) TestMsgAddToTierPosition_DelegatedToSlashedValidator() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	// Advance block so slash happens in a different block than position creation.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))

	// 100% slash via the SDK — may result in exactly zero tokens when the
	// lock amount is a perfect multiple of PowerReduction.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	pos, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated(), "position should still be delegated")

	// Fund owner and attempt AddToTier.
	addAmount := sdkmath.NewInt(500)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)

	// AddToTierPosition internally claims rewards (which should succeed even
	// after the slash), then tries to delegate — which fails if the validator
	// has an invalid exchange rate (zero tokens).
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().Error(err, "AddToTier should fail on validator with invalid exchange rate")
	s.Require().ErrorIs(err, stakingtypes.ErrDelegatorShareExRateInvalid)
}

// TestMsgAddToTierPosition_DelegatedToJailedValidator verifies that AddToTier
// fails when the position is delegated to a jailed (unbonding) validator,
// because delegate() rejects non-bonded validators.
func (s *KeeperSuite) TestMsgAddToTierPosition_DelegatedToJailedValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()

	// Jail the validator — transitions to Unbonding.
	s.jailAndUnbondValidator(valAddr)

	// Fund owner for add.
	addAmount := sdkmath.NewInt(500)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, addAmount)))
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
		Amount:     addAmount,
	})
	s.Require().Error(err, "AddToTier should fail on jailed/unbonding validator")
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)
}
