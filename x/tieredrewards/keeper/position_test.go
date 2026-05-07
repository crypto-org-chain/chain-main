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

var testPositionOwner = sdk.AccAddress([]byte("test_pos_owner______")).String()

func newTestPosition(id uint64, owner string, tierId uint32) types.Position {
	return types.NewPosition(id, owner, tierId, 100, 0, time.Time{}, false, time.Now())
}

func newDelegatedTestPosition(id uint64, owner string, tierId uint32, now time.Time) types.Position {
	return types.NewPosition(id, owner, tierId, 100, 0, now, true, now)
}

func (s *KeeperSuite) seedStakingDelegationForPosition(posId uint64, valAddr sdk.ValAddress, amount sdkmath.Int) {
	s.T().Helper()
	delAddr := types.GetDelegatorAddress(posId)
	_, bondDenom := s.getStakingData()
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, amount))))
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, delAddr, amount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)
}

func (s *KeeperSuite) TestSetAndGetPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	got, err := s.keeper.GetPosition(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, got.Id)
	s.Require().Equal(pos.Owner, got.Owner)
	s.Require().Equal(pos.TierId, got.TierId)
	s.Require().Equal(pos.CreatedAtHeight, got.CreatedAtHeight)
	s.Require().Equal(pos.ExitTriggeredAt, got.ExitTriggeredAt)
	s.Require().Equal(pos.ExitUnlockAt, got.ExitUnlockAt)
	s.Require().Equal(pos.LastEventSeq, got.LastEventSeq)
	s.Require().Equal(pos.LastKnownBonded, got.LastKnownBonded)
}

func (s *KeeperSuite) TestGetPosition_NotFound() {
	_, err := s.keeper.GetPosition(s.ctx, 999)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
}

func (s *KeeperSuite) TestSetPosition_InvalidFails() {
	err := s.keeper.SetPosition(s.ctx, types.Position{}, nil)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestSetPosition_DoesNotIncrementCounterOnUpdate() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	count, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Update same position — counter should not change.
	pos.CreatedAtHeight = 101
	err = s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)
}

func (s *KeeperSuite) TestDeletePosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().NoError(err)

	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos, nil))

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
}

func (s *KeeperSuite) TestDeleteUnsavedPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	err := s.keeper.DeletePosition(s.ctx, pos, nil)
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
	err = s.keeper.SetPosition(s.ctx, pos1, nil)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2, nil)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos3, nil)
	s.Require().NoError(err)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count)

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 2)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Delete one from tier 1
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos1, nil))

	count, err = s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count)

	// Delete last from tier 1
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos2, nil))

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
	err := s.keeper.SetPosition(s.ctx, pos1, nil)
	s.Require().NoError(err)
	err = s.keeper.SetPosition(s.ctx, pos2, nil)
	s.Require().NoError(err)

	ids, err := s.keeper.GetPositionsIdsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().Len(ids, 2)
	s.Require().Contains(ids, uint64(1))
	s.Require().Contains(ids, uint64(2))

	// Delete one and verify index is updated
	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos1, nil))
	ids, err = s.keeper.GetPositionsIdsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().Len(ids, 1)
	s.Require().Equal(uint64(2), ids[0])
}

// --- CreateDelegatedPosition tests ---

func (s *KeeperSuite) TestCreateDelegatedPosition_Basic() {
	s.setupTier(1)
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)

	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	freshAddr := s.fundRandomAddr("stake", sdkmath.NewInt(100_000))

	pos, err := s.keeper.CreateDelegatedPosition(s.ctx, freshAddr.String(), tier, valAddr, false)
	s.Require().NoError(err)
	s.Require().Equal(uint32(1), pos.TierId)
	s.Require().Equal(freshAddr.String(), pos.Owner)
	s.Require().False(pos.IsExiting(s.ctx.BlockTime()))
	s.Require().True(pos.LastKnownBonded, "LastKnownBonded should be true for newly delegated position")
	s.Require().False(pos.LastBonusAccrual.IsZero(), "LastBonusAccrual should be set at creation")
}

func (s *KeeperSuite) TestCreateDelegatedPosition_WithTriggerExitImmediately() {
	s.setupTier(1)
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)

	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	freshAddr := s.fundRandomAddr("stake", sdkmath.NewInt(100_000))

	pos, err := s.keeper.CreateDelegatedPosition(s.ctx, freshAddr.String(), tier, valAddr, true)
	s.Require().NoError(err)
	s.Require().True(pos.IsExiting(s.ctx.BlockTime()))
	s.Require().False(pos.ExitTriggeredAt.IsZero())
	s.Require().False(pos.ExitUnlockAt.IsZero())
	s.Require().True(pos.ExitUnlockAt.After(pos.ExitTriggeredAt))
	s.Require().True(pos.LastKnownBonded)
}

func (s *KeeperSuite) TestCreateDelegatedPosition_IncrementingIds() {
	s.setupTier(1)
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)

	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	freshAddr := s.fundRandomAddr("stake", sdkmath.NewInt(100_000))

	pos1, err := s.keeper.CreateDelegatedPosition(s.ctx, freshAddr.String(), tier, valAddr, false)
	s.Require().NoError(err)

	pos2, err := s.keeper.CreateDelegatedPosition(s.ctx, freshAddr.String(), tier, valAddr, false)
	s.Require().NoError(err)

	s.Require().True(pos2.Id > pos1.Id)
}

func (s *KeeperSuite) TestSetPosition_RejectsInvalidDelegatedPositionState() {
	s.setupTier(1)
	now := s.ctx.BlockTime()
	// No staking delegation seeded → undelegated from keeper's view.
	pos := newDelegatedTestPosition(1, testPositionOwner, 1, now)

	err := s.keeper.SetPosition(s.ctx, pos, nil)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "last_bonus_accrual",
		"error should call out the violated PositionState invariant")
}

func (s *KeeperSuite) TestSetPosition_IncrementsValidatorCounter_NewDelegated() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	// Sanity: validator counter starts at 0.
	count, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), count)

	const posId uint64 = 1
	s.seedStakingDelegationForPosition(posId, valAddr, sdkmath.NewInt(1000))

	now := s.ctx.BlockTime()
	pos := newDelegatedTestPosition(posId, testPositionOwner, 1, now)

	err = s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""})
	s.Require().NoError(err)

	valCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount, "validator counter should increment 0→1")

	tierCount, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), tierCount, "tier counter should increment 0→1")
}

func (s *KeeperSuite) TestSetPosition_SwapValidatorDecrementsAndIncrements() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddrA := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	const posId uint64 = 1
	s.seedStakingDelegationForPosition(posId, valAddrA, sdkmath.NewInt(1000))

	now := s.ctx.BlockTime()
	pos := newDelegatedTestPosition(posId, testPositionOwner, 1, now)

	// Initial write under valA.
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))
	countA, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddrA)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), countA)

	// Flip the live staking delegation to valB.
	delA, err := s.keeper.GetDelegation(s.ctx, posId)
	s.Require().NoError(err)
	s.Require().NotNil(delA)
	s.Require().NoError(s.app.StakingKeeper.RemoveDelegation(s.ctx, *delA))

	valAddrB, _ := s.createSecondValidator()
	s.seedStakingDelegationForPosition(posId, valAddrB, sdkmath.NewInt(1000))

	// setPosition with Previous: valA and live current=valB → decrement A, increment B.
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: valAddrA.String()}))

	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, valAddrA)
	s.Require().Error(err, "valA entry should be removed after swap")

	countB, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddrB)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), countB, "valB should hold the position now")
}

func (s *KeeperSuite) TestSetPosition_NilUpdateSkipsValidatorDiff() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	const posId uint64 = 1
	s.seedStakingDelegationForPosition(posId, valAddr, sdkmath.NewInt(1000))

	now := s.ctx.BlockTime()
	pos := newDelegatedTestPosition(posId, testPositionOwner, 1, now)

	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))
	before, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), before)

	// Re-persist with an unrelated change and a nil update — counter must not budge.
	pos.CreatedAtHeight = 101
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, nil))

	after, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), after, "nil update must not alter validator counter")
}

func (s *KeeperSuite) TestDeletePosition_RejectsWhileDelegated() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	const posId uint64 = 1
	s.seedStakingDelegationForPosition(posId, valAddr, sdkmath.NewInt(1000))

	now := s.ctx.BlockTime()
	pos := newDelegatedTestPosition(posId, testPositionOwner, 1, now)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))

	err := s.keeper.DeletePosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: valAddr.String()})
	s.Require().ErrorIs(err, types.ErrPositionDelegated)

	count, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count, "counter must be unchanged on rejected delete")

	_, err = s.keeper.GetPosition(s.ctx, posId)
	s.Require().NoError(err, "position must still be present in store")
}

func (s *KeeperSuite) TestDeletePosition_NilUpdateSkipsValidatorDiff() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	const posId uint64 = 1
	s.seedStakingDelegationForPosition(posId, valAddr, sdkmath.NewInt(1000))

	now := s.ctx.BlockTime()
	pos := newDelegatedTestPosition(posId, testPositionOwner, 1, now)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))

	// Drop the staking delegation so deletePosition's no-delegation precondition holds.
	del, err := s.keeper.GetDelegation(s.ctx, posId)
	s.Require().NoError(err)
	s.Require().NotNil(del)
	s.Require().NoError(s.app.StakingKeeper.RemoveDelegation(s.ctx, *del))

	s.Require().NoError(s.keeper.DeletePosition(s.ctx, pos, nil))

	count, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count, "nil update must not alter validator counter on delete")
}
