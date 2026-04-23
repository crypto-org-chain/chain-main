package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------------------------------------------------------------------------
// Base rewards
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestBaseRewardsWithdrawal_MarkedOncePerBlock() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	currentHeight := uint64(s.ctx.BlockHeight())

	_, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	lastWithdrawalBlock := s.keeper.GetLastWithdrawalBlock(s.ctx, valAddr)
	s.Require().Equal(currentHeight, lastWithdrawalBlock)

	// Re-read position from store after first claim.
	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	_, _, err = s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	lastWithdrawalBlock = s.keeper.GetLastWithdrawalBlock(s.ctx, valAddr)
	s.Require().Equal(currentHeight, lastWithdrawalBlock, "same-block claim should keep withdrawal marker unchanged")

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	nextHeight := uint64(s.ctx.BlockHeight())

	// Re-read position from store after second claim.
	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	_, _, err = s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	lastWithdrawalBlock = s.keeper.GetLastWithdrawalBlock(s.ctx, valAddr)
	s.Require().Equal(nextHeight, lastWithdrawalBlock, "new block should update withdrawal marker")
}

// TestBaseRewardsPerShare_UpdatesOnClaim verifies that after claiming,
// the position's BaseRewardsPerShare matches the validator's current ratio.
func (s *KeeperSuite) TestBaseRewardsPerShare_UpdatesOnClaim() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	ratioBefore := pos.BaseRewardsPerShare

	// Allocate rewards and advance block so collectDelegationRewards fires.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	_, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Position's checkpoint should have advanced.
	diff, hasNeg := pos.BaseRewardsPerShare.SafeSub(ratioBefore)
	s.Require().False(hasNeg, "BaseRewardsPerShare should not decrease")
	s.Require().False(diff.IsZero(),
		"BaseRewardsPerShare should increase after claim: before=%s, after=%s",
		ratioBefore, pos.BaseRewardsPerShare)

	// Position checkpoint should equal the validator's current ratio.
	validatorRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(validatorRatio, pos.BaseRewardsPerShare,
		"position checkpoint should match validator ratio after claim")
}

// TestBaseRewardsPerShare_UnchangedWhenUndelegated verifies that an
// undelegated position's BaseRewardsPerShare does not change even as the
// validator's ratio keeps increasing.
func (s *KeeperSuite) TestBaseRewardsPerShare_UnchangedWhenUndelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance past exit and undelegate.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Snapshot the undelegated position's checkpoint.
	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	ratioSnapshot := posAfter.BaseRewardsPerShare

	// Allocate more rewards to the validator and advance blocks.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(5000), bondDenom)

	// Claim on the undelegated position -- should be a no-op for base rewards.
	_, _, err = s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{posAfter})
	s.Require().NoError(err)

	posAfter, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(ratioSnapshot, posAfter.BaseRewardsPerShare,
		"undelegated position's BaseRewardsPerShare should not change")
}

// TestValidatorRewardRatio_IncreasesEachBlock verifies that the validator's
// cumulative reward ratio grows with each block when rewards are allocated.
func (s *KeeperSuite) TestValidatorRewardRatio_IncreasesEachBlock() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	var prevRatio sdk.DecCoins

	for block := int64(1); block <= 3; block++ {
		s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
		s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)

		// Re-read position from store for each iteration.
		pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
		s.Require().NoError(err)

		_, _, err = s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
		s.Require().NoError(err)

		ratio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
		s.Require().NoError(err)

		if block > 1 {
			diff, hasNeg := ratio.SafeSub(prevRatio)
			s.Require().False(hasNeg, "ratio should not decrease")
			s.Require().False(diff.IsZero(),
				"ratio should increase: block=%d, prev=%s, curr=%s",
				block, prevRatio, ratio)
		}
		prevRatio = ratio
	}
}

// TestClaimBaseRewards_CorrectAmount verifies the base reward payout matches
// the formula: DelegatedShares * (currentRatio - positionRatio).
func (s *KeeperSuite) TestClaimBaseRewards_CorrectAmount() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	posRatio := pos.BaseRewardsPerShare

	// Allocate known rewards and advance block.
	rewardAmount := sdkmath.NewInt(500)
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, rewardAmount, bondDenom)

	base, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	// Compute expected: shares * (newRatio - oldRatio), truncated.
	newRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)

	delta, _ := newRatio.SafeSub(posRatio)
	expectedDec := delta.MulDecTruncate(pos.DelegatedShares)
	expected, _ := expectedDec.TruncateDecimal()
	expectedBaseAmount := expected.AmountOf(bondDenom)

	s.Require().True(expectedBaseAmount.IsPositive(),
		"expected base reward should be positive")
	s.Require().Equal(base.AmountOf(bondDenom), expectedBaseAmount)
}

// TestClaimBaseRewards_ZeroDelta verifies that when the validator ratio
// has not changed since the position's checkpoint, no base rewards are paid.
func (s *KeeperSuite) TestClaimBaseRewards_ZeroDelta() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Claim once to sync position checkpoint with validator ratio.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	_, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	// Re-read position from store after first claim.
	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	// Don't allocate any new rewards. Claim again on same block --
	// ratio should be unchanged, base reward should be zero.
	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	base, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)
	s.Require().True(base.IsZero(),
		"base rewards should be zero when ratio is unchanged")

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	// Only bonus (if any) should be received, base should be zero.
	// With zero time elapsed since last claim, bonus should also be zero.
	s.Require().Equal(balBefore.Amount, balAfter.Amount,
		"balance should not change when no new rewards accrued")
}

// TestClaimBaseRewards_UndelegatedPosition verifies that claimBaseRewards
// returns empty coins for an undelegated position.
func (s *KeeperSuite) TestClaimBaseRewards_UndelegatedPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	vals, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(posAfter.IsDelegated(), "position should be undelegated")

	// Direct call to ClaimBaseRewards with any ratio -- should return empty.
	currentRatio := sdk.NewDecCoins(sdk.NewDecCoin(bondDenom, sdkmath.NewInt(999)))
	base, err := s.keeper.ClaimBaseRewards(s.ctx, []*types.Position{&posAfter}, delAddr.String(), sdk.MustValAddressFromBech32(vals[0].GetOperator()), currentRatio)
	s.Require().NoError(err)
	s.Require().True(base.IsZero(),
		"claimBaseRewards on undelegated position should return empty")
}

// TestUpdateBaseRewardsPerShare_NoDelegation verifies that when the module
// has no delegation to a validator, updateBaseRewardsPerShare returns empty.
func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_NoDelegation() {
	// Use a validator address that the module has never delegated to.
	fakeVal := sdk.ValAddress([]byte("no_delegation_val___"))

	ratio, err := s.keeper.UpdateBaseRewardsPerShare(s.ctx, fakeVal)
	s.Require().NoError(err)
	s.Require().True(ratio.IsZero(),
		"ratio should be empty when no delegation exists")
}

// TestUpdateBaseRewardsPerShare_ZeroShares verifies that if the module's
// delegation has zero shares, the ratio is not updated.
func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_ZeroShares() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Undelegate to remove the module's delegation shares.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner: delAddr.String(), PositionId: pos.Id,
	})
	s.Require().NoError(err)

	ratio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(500), bondDenom)

	// With no module delegation, updateBaseRewardsPerShare returns empty or
	// the existing ratio without updating.
	updatedRatio, err := s.keeper.UpdateBaseRewardsPerShare(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(ratio, updatedRatio)
}

// TestStaleValidatorRewardRatioReplayed verifies stale validator
// ratio is cleared when module delegation on that validator reaches zero, so a
// later delegation lifecycle cannot replay historical base rewards.
func (s *KeeperSuite) TestStaleValidatorRewardRatioReplayed() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// First lifecycle: claim rewards to leave a non-zero validator ratio.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)

	_, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, delAddr.String(), []types.Position{pos})
	s.Require().NoError(err)

	ratioAfterFirstClaim, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(ratioAfterFirstClaim.IsZero(), "test setup failed: expected a non-zero ratio after first claim")

	// Fully close the first position so module delegation on the validator is removed.
	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr)

	_, err = msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().Error(err, "expected no module delegation after position withdrawal")

	staleRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(staleRatio.IsZero(), "test setup failed: expected historical ratio before re-entry")

	// Second lifecycle: create a fresh position with no new rewards allocated.
	pos2 := s.setupNewTierPosition(lockAmount, false)
	addr2 := sdk.MustAccAddressFromBech32(pos2.Owner)

	ratioAfterReentry, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ratioAfterReentry.IsZero(), "stale validator ratio should be reset on re-entry when no module delegation exists")

	// No new rewards were allocated for this second lifecycle.
	base, _, err := s.keeper.ClaimRewardsForPositions(s.ctx, addr2.String(), []types.Position{pos2})
	s.Require().NoError(err)

	s.Require().True(
		base.AmountOf(bondDenom).IsZero(),
		"second lifecycle claim should not replay historical base rewards when no new rewards were allocated",
	)
}
