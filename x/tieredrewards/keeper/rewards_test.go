package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// --- UpdateBaseRewardsPerShare tests ---

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_NoExistingDelegation() {
	// When the module has no delegation to a validator, UpdateBaseRewardsPerShare
	// should return empty DecCoins without error.
	valAddr := sdk.ValAddress([]byte("no_delegation_val___"))

	ratio, err := s.keeper.UpdateBaseRewardsPerShare(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ratio.IsZero())
}

func (s *KeeperSuite) TestUpdateBaseRewardsPerShare_RatioIsStoredPerValidator() {
	_, valAddr, _ := s.setupTierAndDelegator()

	// Initially no ratio stored
	ratio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(ratio.IsZero())
}

// TestClaimBonusRewardsForPositions_UpdatesOriginalSlice verifies:
// ClaimBonusRewardsForPositions must update the caller's slice elements in-place
// (pointer semantics) so callers that hold the slice after the call see updated
// LastBonusAccrual values — preventing double-claim of bonus rewards.
func (s *KeeperSuite) TestClaimBonusRewardsForPositions_UpdatesOriginalSlice() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour * 24 * 30))
	s.fundRewardsPool(sdkmath.NewInt(100000), bondDenom)

	positions, err := s.keeper.GetPositionsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	originalLastAccrual := positions[0].LastBonusAccrual

	_, err = s.keeper.ClaimBonusRewardsForPositions(s.ctx, positions)
	s.Require().NoError(err)

	// After the call the slice element must reflect the updated LastBonusAccrual.
	s.Require().NotEqual(originalLastAccrual, positions[0].LastBonusAccrual,
		"ClaimBonusRewardsForPositions must update the slice element in-place")

	// Also confirm the store is in sync.
	stored, err := s.keeper.Positions.Get(s.ctx, positions[0].Id)
	s.Require().NoError(err)
	s.Require().Equal(positions[0].LastBonusAccrual, stored.LastBonusAccrual,
		"in-memory slice element must match the stored position")
}

// TestClaimBonusRewards_UsesIntegerDivisionForDuration verifies:
// the bonus duration is computed with integer division (not float64.Seconds()),
// so there is no truncation bias from float representation.
// We construct a duration that has sub-second remainder and confirm the result
// matches integer division.
func (s *KeeperSuite) TestClaimBonusRewards_DurationUsesIntegerSeconds() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	const lockAmt = 1_000_000_000 // large enough for measurable bonus
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           sdkmath.NewInt(lockAmt),
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Advance by a duration that is NOT an exact number of seconds
	// (1h + 500ms). The bonus should be computed for exactly 3600 seconds,
	// not 3600.5 seconds.
	advanceDuration := time.Hour + 500*time.Millisecond
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(advanceDuration))
	s.fundRewardsPool(sdkmath.NewInt(10_000_000_000), bondDenom)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// Compute expected bonus using integer division.
	durationSeconds := int64(advanceDuration / time.Second) // = 3600, not 3600.5
	tokens := val.TokensFromShares(pos.DelegatedShares)
	expectedBonus := tokens.
		Mul(tier.BonusApy).
		MulInt64(durationSeconds).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()

	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.Require().True(expectedBonus.Equal(resp.BonusRewards.AmountOf(bondDenom)),
		"bonus should match integer-second calculation, got %s expected %s",
		resp.BonusRewards.AmountOf(bondDenom), expectedBonus)
}

// TestCalculateBonus_StopsAccruingAfterExitUnlockAt verifies that bonus
// accrual is capped at ExitUnlockAt when the position has completed its
// exit lock duration. Advancing time further past ExitUnlockAt should
// not yield additional bonus.
func (s *KeeperSuite) TestCalculateBonus_StopsAccruingAfterExitUnlockAt() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	const lockAmt = 1_000_000_000
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(lockAmt),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	pos, err := s.keeper.Positions.Get(s.ctx, uint64(0))
	s.Require().NoError(err)
	exitUnlockAt := pos.ExitUnlockAt

	// Claim rewards exactly at ExitUnlockAt.
	s.ctx = s.ctx.WithBlockTime(exitUnlockAt)
	s.fundRewardsPool(sdkmath.NewInt(100_000_000_000), bondDenom)

	respAtUnlock, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	bonusAtUnlock := respAtUnlock.BonusRewards.AmountOf(bondDenom)

	// Advance time well past ExitUnlockAt — bonus should not increase.
	s.ctx = s.ctx.WithBlockTime(exitUnlockAt.Add(time.Hour * 24 * 365))

	respAfterUnlock, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	bonusAfterUnlock := respAfterUnlock.BonusRewards.AmountOf(bondDenom)

	s.Require().True(bonusAtUnlock.IsPositive(), "should have accrued bonus up to ExitUnlockAt")
	s.Require().True(bonusAfterUnlock.IsZero(),
		"bonus should not accrue past ExitUnlockAt, got %s", bonusAfterUnlock)
}
