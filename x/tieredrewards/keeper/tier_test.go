package keeper_test

import (
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
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

	got, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(tier.Id, got.Id)
	s.Require().True(tier.BonusApy.Equal(got.BonusApy))
	s.Require().True(tier.MinLockAmount.Equal(got.MinLockAmount))
	s.Require().Equal(tier.ExitDuration, got.ExitDuration)
	s.Require().Equal(tier.CloseOnly, got.CloseOnly)
}

func (s *KeeperSuite) TestGetTier_NotFound() {
	_, err := s.keeper.Tiers.Get(s.ctx, 999)
	s.Require().Error(err)
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

	got, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().True(sdkmath.LegacyNewDecWithPrec(8, 2).Equal(got.BonusApy))
}

func (s *KeeperSuite) TestSetTier_CloseOnly() {
	tier := newTestTier(1)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	got, err := s.keeper.Tiers.Get(s.ctx, 1)
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

func (s *KeeperSuite) TestKeeperDeleteTier_FailsWithActivePositions() {
	tier := newTestTier(1)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	// Create a position in tier 1
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
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
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	// Remove position
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos))

	// Now delete should succeed
	s.Require().NoError(s.keeper.DeleteTier(s.ctx, 1))

	has, err := s.keeper.HasTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)
}
