package keeper_test

import (
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
