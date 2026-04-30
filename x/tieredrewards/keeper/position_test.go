package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	testPositionOwner = sdk.AccAddress([]byte("test_pos_owner______")).String()
	testPosValidator  = sdk.ValAddress([]byte("test_pos_validator__")).String()
	testPosValidator2 = sdk.ValAddress([]byte("test_pos_validator2_")).String()
)

func newTestPosition(id uint64, owner string, tierId uint32) types.Position {
	del := types.Delegation{
		Validator:    testPosValidator,
		Shares:       sdkmath.LegacyNewDec(1000),
		LastEventSeq: 0,
	}
	return types.NewPosition(id, owner, tierId, sdkmath.ZeroInt(), 100, del, time.Now())
}

func (s *KeeperSuite) TestSetAndGetPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	got, err := s.keeper.GetPosition(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, got.Id)
	s.Require().Equal(pos.Owner, got.Owner)
	s.Require().Equal(pos.TierId, got.TierId)
	s.Require().True(pos.Amount.Equal(got.Amount))
	s.Require().True(pos.DelegatedShares.Equal(got.DelegatedShares))
	s.Require().Equal(pos.CreatedAtHeight, got.CreatedAtHeight)
	s.Require().Equal(pos.Validator, got.Validator)
	s.Require().Equal(pos.ExitTriggeredAt, got.ExitTriggeredAt)
	s.Require().Equal(pos.ExitUnlockAt, got.ExitUnlockAt)
}

func (s *KeeperSuite) TestGetPosition_NotFound() {
	_, err := s.keeper.GetPosition(s.ctx, 999)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
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

	// Update same position — counter should not change.
	// For delegated positions, Amount must remain zero, so we
	// update DelegatedShares instead.
	pos.UpdateDelegatedShares(sdkmath.LegacyNewDec(2000))
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)
}

func (s *KeeperSuite) TestSetPosition_DelegatedNewPositionIncrementsCounter() {
	pos := newTestPosition(1, testPositionOwner, 1)
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

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
}

func (s *KeeperSuite) TestDeletePosition_CleansUnbondingMappings() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	err = s.keeper.UnbondingDelegationMappings.Set(s.ctx, 10, pos.Id)
	s.Require().NoError(err)
	err = s.keeper.UnbondingDelegationMappings.Set(s.ctx, 11, pos.Id)
	s.Require().NoError(err)
	err = s.keeper.RedelegationMappings.Set(s.ctx, 12, pos.Id)
	s.Require().NoError(err)
	err = s.keeper.RedelegationMappings.Set(s.ctx, 13, pos.Id)
	s.Require().NoError(err)
	err = s.keeper.UnbondingDelegationMappings.Set(s.ctx, 14, 999)
	s.Require().NoError(err)
	err = s.keeper.RedelegationMappings.Set(s.ctx, 15, 999)
	s.Require().NoError(err)

	err = s.keeper.DeletePosition(s.ctx, pos)
	s.Require().NoError(err)

	has, err := s.keeper.UnbondingDelegationMappings.Has(s.ctx, 10)
	s.Require().NoError(err)
	s.Require().False(has)
	has, err = s.keeper.UnbondingDelegationMappings.Has(s.ctx, 11)
	s.Require().NoError(err)
	s.Require().False(has)
	has, err = s.keeper.RedelegationMappings.Has(s.ctx, 12)
	s.Require().NoError(err)
	s.Require().False(has)
	has, err = s.keeper.RedelegationMappings.Has(s.ctx, 13)
	s.Require().NoError(err)
	s.Require().False(has)

	// Unrelated mapping must remain.
	has, err = s.keeper.UnbondingDelegationMappings.Has(s.ctx, 14)
	s.Require().NoError(err)
	s.Require().True(has)
	has, err = s.keeper.RedelegationMappings.Has(s.ctx, 15)
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *KeeperSuite) TestDeleteUnsavedPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.DeletePosition(s.ctx, pos)
	s.Require().NoError(err)

	_, err = s.keeper.GetPosition(s.ctx, 1)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
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
	has, err := s.keeper.HasPositionsForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().False(has)

	has, err = s.keeper.HasPositionsForTier(s.ctx, 2)
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
	valAddr := sdk.MustValAddressFromBech32(testPosValidator)

	// Undelegated position — should NOT be in validator index
	pos1 := newTestPosition(1, testPositionOwner, 1)
	pos1.TriggerExit(pos1.CreatedAtTime, newTestTier(1).ExitDuration)
	pos1.ClearDelegation()
	err := s.keeper.SetPosition(s.ctx, pos1)
	s.Require().NoError(err)

	ids, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Empty(ids)

	// Delegated position — should be in validator index
	pos2 := newTestPosition(2, testPositionOwner, 1)
	err = s.keeper.SetPosition(s.ctx, pos2)
	s.Require().NoError(err)

	ids, err = s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)
	s.Require().Equal(uint64(2), ids[0])
}

func (s *KeeperSuite) TestGetPositionsIdsByValidator_Redelegate() {
	valAddr1 := sdk.MustValAddressFromBech32(testPosValidator)
	valAddr2 := sdk.MustValAddressFromBech32(testPosValidator2)

	// Create position delegated to validator 1
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	ids, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr1)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)

	// Redelegate to validator 2
	pos.WithDelegation(types.Delegation{
		Validator:    testPosValidator2,
		Shares:       sdkmath.LegacyNewDec(1000),
		LastEventSeq: 0,
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
	valAddr := sdk.MustValAddressFromBech32(testPosValidator)

	pos1 := newTestPosition(1, testPositionOwner, 1)
	pos2 := newTestPosition(2, testPositionOwner, 1)

	err := s.keeper.SetPosition(s.ctx, pos1)
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
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)

	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(100_000))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, freshAddr, bondDenom)

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, freshAddr, types.GetDelegatorAddress(1), lockAmount))

	delegation := types.Delegation{
		Validator:    valAddr.String(),
		Shares:       sdkmath.LegacyNewDec(1000),
		LastEventSeq: 0,
	}
	pos, err := s.keeper.CreatePosition(s.ctx, freshAddr.String(), tier, sdkmath.ZeroInt(), delegation, false)
	s.Require().NoError(err)
	s.Require().Equal(uint32(1), pos.TierId)
	s.Require().True(pos.Amount.IsZero())
	s.Require().Equal(freshAddr.String(), pos.Owner)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().Equal(delegation.Shares, pos.DelegatedShares)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, freshAddr, bondDenom)
	s.Require().Equal(lockAmount, balBefore.Amount.Sub(balAfter.Amount))
}

func (s *KeeperSuite) TestCreatePosition_WithTriggerExitImmediately() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)

	freshAddr := s.fundRandomAddr("stake", sdkmath.NewInt(100_000))

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, freshAddr, types.GetDelegatorAddress(1), lockAmount))

	delegation := types.Delegation{
		Validator:    valAddr.String(),
		Shares:       sdkmath.LegacyNewDec(1000),
		LastEventSeq: 0,
	}

	pos, err := s.keeper.CreatePosition(s.ctx, freshAddr.String(), tier, sdkmath.ZeroInt(), delegation, true)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().Equal(valAddr.String(), pos.Validator)
	s.Require().Equal(delegation.Shares, pos.DelegatedShares)
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
	s.Require().False(pos.ExitTriggeredAt.IsZero())
	s.Require().False(pos.ExitUnlockAt.IsZero())
	s.Require().True(pos.ExitUnlockAt.After(pos.ExitTriggeredAt))
}

func (s *KeeperSuite) TestCreatePosition_IncrementingIds() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)

	freshAddr := s.fundRandomAddr(bondDenom, sdkmath.NewInt(100_000))

	lockAmount := sdkmath.NewInt(1000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, freshAddr, types.GetDelegatorAddress(1), lockAmount))
	delegation := types.Delegation{
		Validator:    valAddr.String(),
		Shares:       sdkmath.LegacyNewDec(1000),
		LastEventSeq: 0,
	}
	pos1, err := s.keeper.CreatePosition(s.ctx, freshAddr.String(), tier, sdkmath.ZeroInt(), delegation, false)
	s.Require().NoError(err)

	s.Require().NoError(s.keeper.LockFunds(s.ctx, freshAddr, types.GetDelegatorAddress(1), lockAmount))
	pos2, err := s.keeper.CreatePosition(s.ctx, freshAddr.String(), tier, sdkmath.ZeroInt(), delegation, false)
	s.Require().NoError(err)

	s.Require().True(pos2.Id > pos1.Id)
}

// ---------------------------------------------------------------------------
// PositionCountByValidator — increment / decrement via position lifecycle
// ---------------------------------------------------------------------------

// TestPositionCountByValidator_IncrementOnCreate verifies that creating a
// delegated position increments the validator's position count.
func (s *KeeperSuite) TestPositionCountByValidator_IncrementOnCreate() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	count, err := s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Create a second position on the same validator.
	s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)

	count, err = s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count)
}

// TestPositionCountByValidator_DecrementOnDelete verifies that deleting a
// delegated position decrements the validator's position count.
func (s *KeeperSuite) TestPositionCountByValidator_DecrementOnDelete() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	count, err := s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	err = s.keeper.DeletePosition(s.ctx, pos)
	s.Require().NoError(err)

	// Count should be removed (0 → entry deleted from store).
	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().Error(err, "count entry should be removed when it reaches 0")
}

// TestPositionCountByValidator_UndelegatedPosition verifies that undelegated
// positions do not affect the validator position count.
func (s *KeeperSuite) TestPositionCountByValidator_UndelegatedPosition() {
	// Create a delegated position first so we know the validator's count.
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	countBefore, err := s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().NoError(err)

	// Now undelegate it — clearing delegation makes it undelegated.
	pos.ClearDelegation()
	pos.UpdateAmount(sdkmath.NewInt(1000))
	err = s.keeper.SetPosition(s.ctx, pos)
	s.Require().NoError(err)

	// Count should have decreased.
	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().Error(err, "count should be removed after undelegating the only position")

	// Delete the undelegated position — should not affect any validator count.
	err = s.keeper.DeletePosition(s.ctx, pos)
	s.Require().NoError(err)

	_ = countBefore // used above
}
