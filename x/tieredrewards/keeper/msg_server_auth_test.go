package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// --- UpdateParams ---

func (s *KeeperSuite) TestUpdateParams_Success() {
	authority := s.keeper.GetAuthority()
	newParams := types.NewParams(sdkmath.LegacyNewDecWithPrec(5, 2)) // 0.05

	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)

	stored, err := s.keeper.Params.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().True(newParams.TargetBaseRewardsRate.Equal(stored.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestUpdateParams_InvalidAuthority() {
	msg := &types.MsgUpdateParams{
		Authority: "cosmos1invalid",
		Params:    types.DefaultParams(),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "invalid authority")
}

func (s *KeeperSuite) TestUpdateParams_NegativeRate() {
	authority := s.keeper.GetAuthority()
	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    types.NewParams(sdkmath.LegacyNewDec(-1)),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "target base rewards rate cannot be negative")
}

func (s *KeeperSuite) TestUpdateParams_ZeroRate() {
	authority := s.keeper.GetAuthority()
	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    types.NewParams(sdkmath.LegacyZeroDec()),
	}

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)
}

// --- AddTier ---

func (s *KeeperSuite) TestAddTier_Success() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgAddTier{
		Authority: authority,
		Tier:      newTestTier(1),
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().NoError(err)

	// Verify tier was stored
	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint32(1), got.Id)
	s.Require().True(msg.Tier.BonusApy.Equal(got.BonusApy))
}

func (s *KeeperSuite) TestAddTier_InvalidAuthority() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgAddTier{
		Authority: "cosmos1invalid",
		Tier:      newTestTier(1),
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "invalid authority")
}

func (s *KeeperSuite) TestAddTier_AlreadyExists() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msg := &types.MsgAddTier{
		Authority: authority,
		Tier:      tier,
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierAlreadyExists)
}

func (s *KeeperSuite) TestAddTier_InvalidTier() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgAddTier{
		Authority: authority,
		Tier: types.Tier{
			Id:            1,
			ExitDuration:  0, // invalid
			BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
			MinLockAmount: sdkmath.NewInt(1000),
		},
	}

	_, err := msgServer.AddTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "exit duration")
}

// --- UpdateTier ---

func (s *KeeperSuite) TestUpdateTier_Success() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Create tier first
	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	// Update params
	updated := newTestTier(1)
	newBonusApy := sdkmath.LegacyNewDecWithPrec(8, 2)
	newCloseOnly := true
	newExitDuration := time.Hour * 24 * 365 * 2
	newMinLockAmount := sdkmath.NewInt(10000)
	updated.BonusApy = newBonusApy
	updated.CloseOnly = newCloseOnly
	updated.ExitDuration = newExitDuration
	updated.MinLockAmount = newMinLockAmount

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier:      updated,
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().NoError(err)

	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(newBonusApy.Equal(got.BonusApy))
	s.Require().True(newCloseOnly == got.CloseOnly)
	s.Require().True(newExitDuration == got.ExitDuration)
	s.Require().True(newMinLockAmount.Equal(got.MinLockAmount))
}

func (s *KeeperSuite) TestUpdateTier_InvalidAuthority() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgUpdateTier{
		Authority: "cosmos1invalid",
		Tier:      newTestTier(1),
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "invalid authority")
}

func (s *KeeperSuite) TestUpdateTier_NotFound() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier:      newTestTier(999),
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTierNotFound)
}

func (s *KeeperSuite) TestUpdateTier_InvalidTierUpdate() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))
	invalidApy := sdkmath.LegacyNewDec(-1)

	msg := &types.MsgUpdateTier{
		Authority: authority,
		Tier: types.Tier{
			Id:            1,
			ExitDuration:  time.Hour * 24 * 365,
			BonusApy:      invalidApy,
			MinLockAmount: sdkmath.NewInt(1000),
		},
	}

	_, err := msgServer.UpdateTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "bonus apy")
}

func (s *KeeperSuite) TestUpdateTier_BonusApyChange_ClaimsPositions() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance time so bonus accrues.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	baseRewardsDistributed := sdkmath.NewInt(100)
	s.allocateRewardsToValidator(valAddr, baseRewardsDistributed, bondDenom)

	// Compute expected bonus using ComputeSegmentBonus with the old tier.
	posNow, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tokensPerShare, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)
	expBonus := s.keeper.ComputeSegmentBonus(posNow, tier, posNow.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)
	// Lock amount equals genesis delegation, so tier module holds half the total stake.
	expBase := baseRewardsDistributed.Quo(sdkmath.NewInt(2))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Update tier with new BonusApy
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	updated := newTestTier(1)
	updated.BonusApy = sdkmath.LegacyNewDecWithPrec(8, 2)
	_, err = msgServer.UpdateTier(s.ctx, &types.MsgUpdateTier{
		Authority: s.keeper.GetAuthority(),
		Tier:      updated,
	})
	s.Require().NoError(err)

	// Owner should have received exactly the expected base + bonus at the old rate.
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	actualRewards := balAfter.Amount.Sub(balBefore.Amount)
	expectedTotal := expBase.Add(expBonus)
	s.Require().True(expectedTotal.Equal(actualRewards),
		"rewards mismatch: expected %s (base %s + bonus %s), got %s",
		expectedTotal, expBase, expBonus, actualRewards)

	// Position LastBonusAccrual should be advanced to current block time.
	posNow, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(s.ctx.BlockTime(), posNow.LastBonusAccrual)

	// Subsequent claim should yield zero bonus (window was already consumed).
	respClaim, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:       delAddr.String(),
		PositionIds: []uint64{pos.Id},
	})
	s.Require().NoError(err)
	s.Require().True(respClaim.BonusRewards.IsZero(), "bonus should already be claimed by UpdateTier")
}

func (s *KeeperSuite) TestUpdateTier_NonApyChange_NoClaim() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	posBefore, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Advance time so bonus would accrue.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	_, bondDenom := s.getStakingData()
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// Update only CloseOnly — BonusApy stays the same.
	updated := newTestTier(1)
	updated.CloseOnly = true
	_, err = msgServer.UpdateTier(s.ctx, &types.MsgUpdateTier{
		Authority: s.keeper.GetAuthority(),
		Tier:      updated,
	})
	s.Require().NoError(err)

	// No rewards should have been paid.
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(balBefore.Amount, balAfter.Amount)

	// LastBonusAccrual should be unchanged.
	posAfter, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(posBefore.LastBonusAccrual, posAfter.LastBonusAccrual)
}

func (s *KeeperSuite) TestUpdateTier_BonusApyChange_NoPositions() {
	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	updated := newTestTier(1)
	newBonusApy := sdkmath.LegacyNewDecWithPrec(8, 2)
	updated.BonusApy = newBonusApy
	_, err := msgServer.UpdateTier(s.ctx, &types.MsgUpdateTier{
		Authority: s.keeper.GetAuthority(),
		Tier:      updated,
	})
	s.Require().NoError(err)

	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(newBonusApy.Equal(got.BonusApy))
}

func (s *KeeperSuite) TestUpdateTier_BonusApyChange_InsufficientPool() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance time so bonus accrues.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Update tier with new BonusApy — should fail due to insufficient pool.
	tier := newTestTier(1)
	initialBonusApy := tier.BonusApy
	updatedBonusApy := sdkmath.LegacyNewDecWithPrec(8, 2)
	tier.BonusApy = updatedBonusApy
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateTier(s.ctx, &types.MsgUpdateTier{
		Authority: s.keeper.GetAuthority(),
		Tier:      tier,
	})
	s.Require().ErrorIs(err, types.ErrInsufficientBonusPool)

	// Tier should NOT have been updated (tx failed).
	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(initialBonusApy.Equal(got.BonusApy), "tier should still have old APY")
}

// --- DeleteTier ---

func (s *KeeperSuite) TestDeleteTier_Success() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        1,
	}

	_, err := msgServer.DeleteTier(s.ctx, msg)
	s.Require().NoError(err)

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}

func (s *KeeperSuite) TestDeleteTier_InvalidAuthority() {
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgDeleteTier{
		Authority: "cosmos1invalid",
		Id:        1,
	}

	_, err := msgServer.DeleteTier(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "invalid authority")
}

func (s *KeeperSuite) TestDeleteTier_NotFound() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        999,
	}

	_, err := msgServer.DeleteTier(s.ctx, msg)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestDeleteTier_FailsWithActivePositions() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	// Create a position in tier 1
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        1,
	}

	_, err = msgServer.DeleteTier(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrTierHasActivePositions)

	// Tier should still exist
	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestDeleteTier_SucceedsAfterPositionsRemoved() {
	authority := s.keeper.GetAuthority()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	// Remove the position
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos, nil))

	msg := &types.MsgDeleteTier{
		Authority: authority,
		Id:        1,
	}

	_, err = msgServer.DeleteTier(s.ctx, msg)
	s.Require().NoError(err)

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}
