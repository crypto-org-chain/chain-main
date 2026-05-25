package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	collections "cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMsgTierUndelegate_Basic() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	// Create delegated + exit-triggered position

	// Fund the rewards pool so bonus claim doesn't fail
	s.fundRewardsPool(sdkmath.NewInt(10000), bondDenom)

	s.advancePastExitDuration()
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero())

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().False(pos.IsDelegated(), "position should not be delegated after undelegate")
	s.Require().Nil(pos.Delegation, "delegation should be cleared")

	_, err = s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().ErrorIs(err, collections.ErrNotFound)

	// The position should have a pending unbonding delegation entry in staking.
	ubds, err := s.app.StakingKeeper.GetUnbondingDelegations(s.ctx, types.GetDelegatorAddress(pos.Id), 1)
	s.Require().NoError(err)
	s.Require().NotEmpty(ubds, "position should have a pending unbonding delegation entry")

	// No redelegating-position mapping should exist for a plain undelegate.
	isRedelegating, err := s.keeper.IsRedelegating(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(isRedelegating, "undelegate must not populate the redelegating-position mapping")
}

func (s *KeeperSuite) TestMsgTierUndelegate_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// First undelegate succeeds
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	// Second undelegate should fail with ErrPositionNotDelegated
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgTierUndelegate_ExitNotTriggered() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

// TestMsgTierUndelegate_ExitDurationNotReached verifies that TierUndelegate is
// rejected when exit is triggered but duration has not elapsed.
func (s *KeeperSuite) TestMsgTierUndelegate_ExitDurationNotReached() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgTierUndelegate_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	wrongAddr := sdk.AccAddress([]byte("wrong_owner_________"))
	s.advancePastExitDuration()
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      wrongAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

// TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating verifies that TierUndelegate
// claims pending rewards before undelegating. A subsequent ClaimTierRewards would fail
// (position no longer delegated), but the balance increase confirms rewards were paid.
func (s *KeeperSuite) TestMsgTierUndelegate_ClaimsRewardsBeforeUndelegating() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Advance block so delegation starting period is finalized, then allocate rewards.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.advancePastExitDuration()
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)

	// TierUndelegate internally claims rewards.
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	balAfterUndelegate := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	s.Require().True(balAfterUndelegate.Amount.GT(balBefore.Amount), "rewards should be paid during undelegate")
}

// TestMsgTierUndelegate_UpdatesAmount verifies that TierUndelegate
// updates the amount with the staking module's exact return amount.
func (s *KeeperSuite) TestMsgTierUndelegate_UpdatesAmount() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)

	positions, err := s.keeper.GetPositionStatesByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos, err = s.keeper.GetPositionState(s.ctx, positions[0].Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())

	s.fundRewardsPool(sdkmath.NewInt(100_000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	resp, err := msgServer.WithdrawFromTier(s.ctx, &types.MsgWithdrawFromTier{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().Equal(lockAmount.String(), resp.Amount.AmountOf(bondDenom).String(),
		"withdrawn amount should equal the locked amount")
}

func (s *KeeperSuite) TestMsgTierUndelegate_AfterBondedSlash_Succeeds() {
	lockAmount := sdkmath.NewInt(10_000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	addr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	positions, err := s.keeper.GetPositionStatesByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	pos, err = s.keeper.GetPositionState(s.ctx, positions[0].Id)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(1, 2)) // 1%

	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.advancePastExitDuration()

	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero(), "undelegation should still succeed after a bonded slash")
}

// TestMsgTierUndelegate_BondedZeroAmount verifies that TierUndelegate succeeds
// on a delegated position with zero amount (100%% bonded slash). The staking
// layer returns zero tokens and the position is cleanly undelegated.
func (s *KeeperSuite) TestMsgTierUndelegate_BondedZeroAmount() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Slash validator 100% to zero out position amount via hook.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	s.advancePastExitDuration()

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(s.getPositionAmount(pos).IsZero(), "position amount should be zero")
	s.Require().False(pos.IsDelegated(), "position should be undelegated")
}

// TestMsgTierUndelegate_TierCloseOnly_Succeeds verifies that TierUndelegate is
// NOT blocked by CloseOnly — exit-path messages must always succeed.
func (s *KeeperSuite) TestMsgTierUndelegate_TierCloseOnly_Succeeds() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	s.advancePastExitDuration()
	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)

	// Set tier to CloseOnly.
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	// TierUndelegate — should succeed despite CloseOnly.
	resp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.Require().False(resp.CompletionTime.IsZero(), "undelegation should succeed on CloseOnly tier")

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated(), "position should be undelegated")
}

// TestMsgTierUndelegate_ManyPositionsSameValidator verifies that many tier
// positions on the same validator can all undelegate concurrently.
func (s *KeeperSuite) TestMsgTierUndelegate_ManyPositionsSameValidator() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Deliberately exceed cosmos-sdk's default MaxEntries (7) per (delegator,
	// validator) pair — any N > 7 demonstrates the benefit.
	const numPositions = 10
	lockAmount := sdkmath.NewInt(1_000)

	owners := make([]sdk.AccAddress, numPositions)
	positionIds := make([]uint64, numPositions)
	for i := 0; i < numPositions; i++ {
		owner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
		s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, owner,
			sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
		resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
			Owner:                  owner.String(),
			Id:                     1,
			Amount:                 lockAmount,
			ValidatorAddress:       valAddr.String(),
			TriggerExitImmediately: true,
		})
		s.Require().NoError(err)
		owners[i] = owner
		positionIds[i] = resp.PositionId
	}

	s.advancePastExitDuration()

	for i := 0; i < numPositions; i++ {
		_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
			Owner:      owners[i].String(),
			PositionId: positionIds[i],
		})
		s.Require().NoError(err,
			"position %d (of %d) must not be blocked by a shared MaxEntries cap — each position has its own staking delegator",
			i+1, numPositions)
	}

	for i := 0; i < numPositions; i++ {
		pos, err := s.keeper.GetPositionState(s.ctx, positionIds[i])
		s.Require().NoError(err)
		s.Require().False(pos.IsDelegated(), "position %d should be undelegated", i)
	}
}
