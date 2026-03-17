package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	testPositionOwner = sdk.AccAddress([]byte("test_pos_owner______")).String()
	testPosValidator  = sdk.ValAddress([]byte("test_pos_validator__")).String()
	testPosValidator2 = sdk.ValAddress([]byte("test_pos_validator2_")).String()
)

func newTestPosition(id uint64, owner string, tierId uint32) types.Position {
	return types.NewBasePosition(id, owner, tierId, sdkmath.NewInt(1000), 100, time.Now())
}

func (s *KeeperSuite) TestSetAndGetPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	got, err := s.keeper.Positions.Get(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, got.Id)
	s.Require().Equal(pos.Owner, got.Owner)
	s.Require().Equal(pos.TierId, got.TierId)
	s.Require().True(pos.Amount.Equal(got.Amount))
}

func (s *KeeperSuite) TestGetPosition_NotFound() {
	_, err := s.keeper.Positions.Get(s.ctx, 999)
	s.Require().ErrorIs(err, collections.ErrNotFound)
}

func (s *KeeperSuite) TestSetPosition_InvalidFails() {
	pos := types.Position{}
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestSetPosition_UpdateDoesNotIncrementCounter() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Update same position — counter should not change
	pos.UpdateAmount(sdkmath.NewInt(2000))
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)
}

func (s *KeeperSuite) TestSetPosition_DelegatedNewPositionIncrementsCounter() {
	pos := newTestPosition(1, testPositionOwner, 1)
	pos.WithDelegation(types.Delegation{
		Validator: testPosValidator,
		Shares:    sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
		},
	}, time.Now())
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)
}

func (s *KeeperSuite) TestDeletePosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos))

	_, err = s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().ErrorIs(err, collections.ErrNotFound)
}

func (s *KeeperSuite) TestDeleteUnsavedPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.DeletePosition(s.ctx, pos)
	s.Require().NoError(err)

	_, err = s.keeper.Positions.Get(s.ctx, 1)
	s.Require().ErrorIs(err, collections.ErrNotFound)
}

func (s *KeeperSuite) TestPositionCountByTier() {
	pos1 := newTestPosition(1, testPositionOwner, 1)
	pos2 := newTestPosition(2, testPositionOwner, 1)
	pos3 := newTestPosition(3, testPositionOwner, 2)

	// Initially zero
	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count)

	// Create positions
	err = s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos3)
	s.Require().NoError(err)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 2)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Delete one from tier 1
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos1))

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Delete last from tier 1
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos2))

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count)

	// HasActivePositionsForTier should reflect
	has, err := s.keeper.HasActivePositionsForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)

	has, err = s.keeper.HasActivePositionsForTier(s.ctx, 2)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestGetPositionsIdsByOwner() {
	owner := sdk.AccAddress([]byte("test_pos_owner______"))

	pos1 := newTestPosition(1, owner.String(), 1)
	pos2 := newTestPosition(2, owner.String(), 1)
	err := s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)

	ids, err := s.keeper.GetPositionsIdsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().Len(ids, 2)
	s.Require().Contains(ids, uint64(1))
	s.Require().Contains(ids, uint64(2))

	// Delete one and verify index is updated
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos1))
	ids, err = s.keeper.GetPositionsIdsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)
	s.Require().Equal(uint64(2), ids[0])
}

func (s *KeeperSuite) TestGetPositionsIdsByValidator() {
	valAddr, err := sdk.ValAddressFromBech32(testPosValidator)
	s.Require().NoError(err)

	// Undelegated position — should NOT be in validator index
	pos1 := newTestPosition(1, testPositionOwner, 1)
	err = s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)

	ids, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Empty(ids)

	// Delegated position — should be in validator index
	pos2 := newTestPosition(2, testPositionOwner, 1)
	pos2.WithDelegation(types.Delegation{
		Validator: testPosValidator,
		Shares:    sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
		},
	}, time.Now())
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)

	ids, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)
	s.Require().Equal(uint64(2), ids[0])
}

func (s *KeeperSuite) TestGetPositionsIdsByValidator_Redelegate() {
	valAddr1, err := sdk.ValAddressFromBech32(testPosValidator)
	s.Require().NoError(err)
	valAddr2, err := sdk.ValAddressFromBech32(testPosValidator2)
	s.Require().NoError(err)

	// Create position delegated to validator 1
	pos := newTestPosition(1, testPositionOwner, 1)
	pos.WithDelegation(types.Delegation{
		Validator: testPosValidator,
		Shares:    sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
		},
	}, time.Now())
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	ids, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr1)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)

	// Redelegate to validator 2
	pos.WithDelegation(types.Delegation{
		Validator: testPosValidator2,
		Shares:    sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
		},
	}, time.Now())
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	// Validator 1 should have no positions
	ids, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr1)
	s.Require().NoError(err)
	s.Require().Empty(ids)

	// Validator 2 should have the position
	ids, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr2)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)
	s.Require().Equal(uint64(1), ids[0])
}

func (s *KeeperSuite) TestGetPositionsByOwner() {
	owner := sdk.AccAddress([]byte("test_pos_owner______"))

	pos1 := newTestPosition(1, owner.String(), 1)
	pos2 := newTestPosition(2, owner.String(), 1)
	err := s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().Len(positions, 2)
}

func (s *KeeperSuite) TestGetPositionsByValidator() {
	valAddr, err := sdk.ValAddressFromBech32(testPosValidator)
	s.Require().NoError(err)

	pos1 := newTestPosition(1, testPositionOwner, 1)
	pos1.WithDelegation(types.Delegation{
		Validator: testPosValidator,
		Shares:    sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
		},
	}, time.Now())

	pos2 := newTestPosition(2, testPositionOwner, 1)
	pos2.WithDelegation(types.Delegation{
		Validator: testPosValidator,
		Shares:    sdkmath.LegacyNewDec(500),
		BaseRewardsPerShare: sdk.DecCoins{
			sdk.NewDecCoinFromDec("basecro", sdkmath.LegacyMustNewDecFromStr("0.5")),
		},
	}, time.Now())

	err = s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 2)
}

func (s *KeeperSuite) TestGetPositionsByIds() {
	pos1 := newTestPosition(1, testPositionOwner, 1)
	pos2 := newTestPosition(2, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByIds(s.ctx, []uint64{1, 2})
	s.Require().NoError(err)
	s.Require().Len(positions, 2)

	// Non existent ID should not throw error
	positions, err = s.keeper.GetPositionsByIds(s.ctx, []uint64{1, 999})
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	positions, err = s.keeper.GetPositionsByIds(s.ctx, []uint64{})
	s.Require().NoError(err)
	s.Require().Empty(positions)
}

// --- CreatePosition tests ---

func (s *KeeperSuite) TestCreatePosition_Basic() {
	delAddr, _, bondDenom := s.setupTierAndDelegator()
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))

	pos, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, nil, false)
	s.Require().NoError(err)
	s.Require().Equal(uint32(1), pos.TierId)
	s.Require().True(lockAmount.Equal(pos.Amount))
	s.Require().Equal(delAddr.String(), pos.Owner)
	s.Require().False(pos.IsDelegated())
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().Equal(lockAmount, balBefore.Amount.Sub(balAfter.Amount))
}

func (s *KeeperSuite) TestCreatePosition_WithValidator() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))

	delegation := &types.Delegation{
		Validator:           valAddr.String(),
		Shares:              sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{},
	}

	pos, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, delegation, false)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().True(pos.BaseRewardsPerShare.IsZero())
}

func (s *KeeperSuite) TestCreatePosition_WithValidatorAndBaseRewardsPerShare() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))

	rps := sdk.DecCoins{sdk.NewDecCoinFromDec(bondDenom, sdkmath.LegacyMustNewDecFromStr("0.5"))}

	delegation := &types.Delegation{
		Validator:           valAddr.String(),
		Shares:              sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: rps,
	}

	pos, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, delegation, false)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().True(pos.BaseRewardsPerShare.Equal(rps))
}

func (s *KeeperSuite) TestCreatePosition_WithTriggerExitImmediately() {
	delAddr, _, _ := s.setupTierAndDelegator()
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))

	pos, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, nil, true)
	s.Require().NoError(err)
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
	s.Require().False(pos.ExitTriggeredAt.IsZero())
	s.Require().False(pos.ExitUnlockAt.IsZero())
	s.Require().True(pos.ExitUnlockAt.After(pos.ExitTriggeredAt))
}

func (s *KeeperSuite) TestCreatePosition_IncrementingIds() {
	delAddr, _, _ := s.setupTierAndDelegator()
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))
	pos1, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, nil, false)
	s.Require().NoError(err)

	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))
	pos2, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, nil, false)
	s.Require().NoError(err)

	s.Require().True(pos2.Id > pos1.Id)
}
