package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newTestTier(id uint32) types.Tier {
	return types.Tier{
		Id:            id,
		ExitDuration:  time.Hour * 24 * 365,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(1000),
	}
}

func (s *KeeperSuite) TestSetAndGetTier() {
	tier := newTestTier(1)
	err := s.keeper.SetTier(s.ctx, tier)
	s.Require().NoError(err)

	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(tier.Id, got.Id)
	s.Require().True(tier.BonusApy.Equal(got.BonusApy))
	s.Require().True(tier.MinLockAmount.Equal(got.MinLockAmount))
	s.Require().Equal(tier.ExitDuration, got.ExitDuration)
	s.Require().Equal(tier.CloseOnly, got.CloseOnly)
}

func (s *KeeperSuite) TestGetTier_NotFound() {
	_, err := s.keeper.GetTier(s.ctx, 999)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTierNotFound)
}

func (s *KeeperSuite) TestSetTier_InvalidFails() {
	tier := types.Tier{}
	err := s.keeper.SetTier(s.ctx, tier)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestSetTier_UpdateExisting() {
	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	// Update bonus APY
	tier.BonusApy = sdkmath.LegacyNewDecWithPrec(8, 2)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(sdkmath.LegacyNewDecWithPrec(8, 2).Equal(got.BonusApy))
}

func (s *KeeperSuite) TestSetTier_CloseOnly() {
	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	got, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(got.IsCloseOnly())
}

func (s *KeeperSuite) TestHasTier() {
	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)

	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))

	has, err = s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestDeleteTier() {
	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	s.Require().NoError(s.keeper.DeleteTier(s.ctx, 1))

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}

func (s *KeeperSuite) TestKeeperDeleteTier_FailsWithPositions() {
	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	// Create a position in tier 1
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	// Delete should fail
	err = s.keeper.DeleteTier(s.ctx, 1)
	s.Require().ErrorIs(err, types.ErrTierHasActivePositions)

	// Tier should still exist
	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestKeeperDeleteTier_SucceedsAfterPositionsRemoved() {
	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	// Remove position
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos, nil))

	// Now delete should succeed
	s.Require().NoError(s.keeper.DeleteTier(s.ctx, 1))

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}

// ---------------------------------------------------------------------------
// Tier position count helpers
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestHasPositionsForTier_FalseWhenEmpty() {
	s.setupTier(1)

	has, err := s.keeper.HasPositionsForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}

func (s *KeeperSuite) TestHasPositionsForTier_TrueWhenPositionExists() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)

	has, err := s.keeper.HasPositionsForTier(s.ctx, pos.TierId)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestGetPositionCountForTier_ZeroWhenEmpty() {
	s.setupTier(1)

	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count)
}

func (s *KeeperSuite) TestIncreaseDecreasePositionCountForTier() {
	s.setupTier(1)

	// Increase twice.
	s.Require().NoError(s.keeper.IncreasePositionCountForTier(s.ctx, 1))
	s.Require().NoError(s.keeper.IncreasePositionCountForTier(s.ctx, 1))

	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count)

	// Decrease once → 1.
	s.Require().NoError(s.keeper.DecreasePositionCountForTier(s.ctx, 1))
	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Decrease again → 0, entry removed from store.
	s.Require().NoError(s.keeper.DecreasePositionCountForTier(s.ctx, 1))
	_, err = s.keeper.PositionCountByTier.Get(s.ctx, uint32(1))
	s.Require().Error(err, "count entry should be removed when reaching 0")

	// Decrease on 0 is a no-op.
	s.Require().NoError(s.keeper.DecreasePositionCountForTier(s.ctx, 1))
}

// ---------------------------------------------------------------------------
// Validator position count helpers
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestGetPositionCountForValidator_ZeroWhenEmpty() {
	valAddr := sdk.ValAddress([]byte("val_empty___________"))

	count, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count)
}

func (s *KeeperSuite) TestIncreaseDecreaseValidatorPositionCount() {
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	// Increase twice.
	s.Require().NoError(s.keeper.IncreasePositionCountForValidator(s.ctx, valAddr))
	s.Require().NoError(s.keeper.IncreasePositionCountForValidator(s.ctx, valAddr))

	count, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count)

	// Decrease once → 1.
	s.Require().NoError(s.keeper.DecreasePositionCountForValidator(s.ctx, valAddr))
	count, err = s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Decrease again → 0, entry removed from store.
	s.Require().NoError(s.keeper.DecreasePositionCountForValidator(s.ctx, valAddr))
	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().Error(err, "count entry should be removed when reaching 0")

	// Decrease on 0 is a no-op.
	s.Require().NoError(s.keeper.DecreasePositionCountForValidator(s.ctx, valAddr))
}
