package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestMsgExitTierWithDelegation_Basic() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, resp.PositionId)
	s.Require().Equal(pos.Amount, resp.TransferredAmount)
	s.Require().Equal(pos.DelegatedShares, resp.TransferredShares)
	s.Require().True(resp.FullExit, "full exit should be true when entire position is transferred")

	// Position should be deleted.
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)

	// Owner should have a staking delegation on the same validator.
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(pos.DelegatedShares, del.Shares, "owner should hold the full delegation shares")
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_Partial() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	exitAmount := lockAmount.Quo(sdkmath.NewInt(2))
	exitShares := pos.DelegatedShares.Quo(sdkmath.LegacyNewDec(2))

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     exitAmount,
	})
	s.Require().NoError(err)
	s.Require().Equal(exitAmount, resp.TransferredAmount)
	s.Require().Equal(exitShares, resp.TransferredShares)
	s.Require().False(resp.FullExit, "partial exit should not be full exit")

	// Position should still exist with reduced amount.
	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.IsDelegated(), "position should remain delegated after partial exit")
	s.Require().Equal(pos.DelegatedShares.Sub(exitShares), posAfter.DelegatedShares, "position shares should be reduced by the amount of exited shares")

	// Owner should have a staking delegation.
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(exitShares, del.Shares, "owner should hold the exited shares")
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_PartialThenFull() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	exitAmount := lockAmount.Quo(sdkmath.NewInt(2))
	exitShares := pos.DelegatedShares.Quo(sdkmath.LegacyNewDec(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// First: partial exit.
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     exitAmount,
	})
	s.Require().NoError(err)

	// Owner should have a staking delegation.
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(exitShares, del.Shares)

	// Position still exists.
	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Equal(pos.Amount.Sub(exitAmount), posAfter.Amount, "position amount should be reduced by the amount of exited amount")
	s.Require().Equal(pos.DelegatedShares.Sub(exitShares), posAfter.DelegatedShares, "position shares should be reduced by the amount of exited shares")
	s.Require().NoError(err)

	// Second: exit the remainder.
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     posAfter.Amount,
	})
	s.Require().NoError(err)

	// Position should be deleted.
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)

	// Owner should have the full delegation.
	del, err = s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(pos.DelegatedShares, del.Shares, "owner should hold the full delegation shares")
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_PartialBelowMinLock() {
	// MinLockAmount for test tier is 1000. Lock 1500, try to exit 600 → remaining 900 < 1000.
	lockAmount := sdkmath.NewInt(1500)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(600), // remaining = ~900 < MinLockAmount(1000)
	})
	s.Require().ErrorIs(err, types.ErrMinLockAmountNotMet)

	// Position should be unchanged.
	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.Equal(pos.Amount), "position amount should be unchanged after failed partial exit")
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	s.advancePastExitDuration()

	wrongOwner := sdk.AccAddress([]byte("wrong_owner_________")).String()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      wrongOwner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().ErrorIs(err, types.ErrNotPositionOwner)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	// Undelegate first so position is no longer delegated.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_ExitNotTriggered() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_ExitDurationNotReached() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	// Do NOT advance past exit duration.

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_AmountExceedsPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount.Add(sdkmath.NewInt(1)),
	})
	s.Require().ErrorIs(err, types.ErrInvalidAmount)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_AmountZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(0),
	})
	s.Require().Error(err)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_TierCloseOnly_Succeeds() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	// Set tier to close-only.
	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)
	tier.CloseOnly = true
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().NoError(err, "close-only should not block exit")

	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_ActiveRedelegation() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Redelegate to create an active redelegation entry.
	dstValAddr, _ := s.createSecondValidator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	s.advancePastExitDuration()

	// Should fail because position has an active redelegation.
	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     pos.Amount,
	})
	s.Require().ErrorIs(err, types.ErrActiveRedelegation)
}

// TestMsgExitTierWithDelegation_PartialAfterSlash verifies that a partial exit
// after a validator slash correctly tracks remaining shares. The slash creates a
// non-1:1 exchange rate so the Unbond→Delegate round-trip changes the
// tokens-per-share ratio. The position's remaining DelegatedShares must equal
// the shares actually removed from the module (not the owner's new shares).
func (s *KeeperSuite) TestMsgExitTierWithDelegation_PartialAfterSlash() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Slash 10% to create a non-1:1 exchange rate.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))

	// Re-read position after slash hook updated it.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())

	s.advancePastExitDuration()

	// Partial exit: half the position amount.
	exitAmount := pos.Amount.Quo(sdkmath.NewInt(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     exitAmount,
	})
	s.Require().NoError(err)

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(posAfter.IsDelegated())

	// The position's DelegatedShares must match what the module actually has
	// for this position. Query the module's total delegation on the validator.
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	moduleDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().NoError(err)

	// Module delegation includes only this position (single position on this validator).
	s.Require().True(posAfter.DelegatedShares.Equal(moduleDel.Shares),
		"position DelegatedShares (%s) must equal module's actual delegation shares (%s)",
		posAfter.DelegatedShares, moduleDel.Shares)

	// A subsequent full exit (TierUndelegate) using the stored shares must succeed.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err, "TierUndelegate should succeed with correct remaining shares")
}
