package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMsgExitTierWithDelegation_Basic() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	// Compute token value from shares for the exit amount.
	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)
	s.Require().True(tokenValue.IsPositive())

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, resp.PositionId)
	s.Require().Equal(tokenValue, resp.TransferredAmount)
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

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	exitAmount := tokenValue.Quo(sdkmath.NewInt(2))
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

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	exitAmount := tokenValue.Quo(sdkmath.NewInt(2))
	exitShares := pos.DelegatedShares.Quo(sdkmath.LegacyNewDec(2))
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// First: partial exit.
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
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
	s.Require().NoError(err)
	s.Require().True(posAfter.Amount.IsZero(), "delegated position Amount should remain zero")
	s.Require().Equal(pos.DelegatedShares.Sub(exitShares), posAfter.DelegatedShares, "position shares should be reduced by the amount of exited shares")

	// Second: exit the remainder using token value from remaining shares.
	remainingTokenValue, err := s.keeper.PositionTokenValue(s.ctx, posAfter)
	s.Require().NoError(err)
	s.Require().True(remainingTokenValue.IsPositive())

	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     remainingTokenValue,
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
	s.Require().True(posAfter.DelegatedShares.Equal(pos.DelegatedShares), "position shares should be unchanged after failed partial exit")
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_WrongOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	s.advancePastExitDuration()

	wrongOwner := sdk.AccAddress([]byte("wrong_owner_________")).String()

	// Compute token value for the exit amount.
	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      wrongOwner,
		PositionId: pos.Id,
		Amount:     tokenValue,
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

	posAfter, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     posAfter.Amount,
	})
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_ExitNotTriggered() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().ErrorIs(err, types.ErrExitNotTriggered)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_ExitDurationNotReached() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	// Do NOT advance past exit duration.

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().ErrorIs(err, types.ErrExitLockDurationNotReached)
}

func (s *KeeperSuite) TestMsgExitTierWithDelegation_AmountExceedsPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	s.advancePastExitDuration()

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue.Add(sdkmath.NewInt(1)),
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

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
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

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().ErrorIs(err, types.ErrActiveRedelegation)
}

// TestMsgExitTierWithDelegation_PartialAfterSlash verifies that a partial exit
// after a validator slash correctly tracks remaining shares. The slash creates a
// non-1:1 exchange rate so the Unbond->Delegate round-trip changes the
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

	// Partial exit: half the token value.
	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)
	exitAmount := tokenValue.Quo(sdkmath.NewInt(2))
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

	posDelAddr := types.GetDelegatorAddress(posAfter.Id)
	posStakingDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)

	s.Require().True(posAfter.DelegatedShares.Equal(posStakingDel.Shares),
		"position DelegatedShares (%s) must equal actual delegation shares (%s)",
		posAfter.DelegatedShares, posStakingDel.Shares)

	// A subsequent full exit (TierUndelegate) using the stored shares must succeed.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err, "TierUndelegate should succeed with correct remaining shares")
}

// TestMsgExitTierWithDelegation_BondedSlashZero verifies that ExitTierWithDelegation
// fails on a position slashed to zero while bonded (S11-e). The position is still
// delegated (worthless shares) but token value=0, so any positive exit amount exceeds it.
func (s *KeeperSuite) TestMsgExitTierWithDelegation_BondedSlashZero() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(1000), true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	// Slash 100% — position amount goes to zero but remains delegated.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.Amount.IsZero(), "position amount should be zero after 100% slash")
	s.Require().True(pos.IsDelegated(), "position should still be delegated")

	s.advancePastExitDuration()

	// Any positive amount exceeds the token value (0) -> ErrInvalidAmount.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     sdkmath.NewInt(1),
	})
	s.Require().ErrorIs(err, types.ErrInvalidAmount)
}

// TestMsgExitTierWithDelegation_FullExitAfterSlash verifies that a full exit
// after a validator slash (non-1:1 exchange rate) works correctly. The user
// passes the post-slash token value and ExitWithFullDelegation returns true,
// so all DelegatedShares are used directly (no ValidateUnbondAmount truncation).
func (s *KeeperSuite) TestMsgExitTierWithDelegation_FullExitAfterSlash() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Slash 10% to create a non-1:1 exchange rate.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(10, 2))

	// Re-read position after slash hook.
	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())

	// Compute token value from shares (post-slash).
	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)
	s.Require().True(tokenValue.LT(lockAmount), "token value should be reduced after slash")

	s.advancePastExitDuration()

	// Full exit using post-slash token value.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().NoError(err)
	s.Require().True(resp.FullExit)
	s.Require().True(resp.TransferredAmount.IsPositive())

	// Position should be deleted.
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)

	// Owner should have a staking delegation.
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(del.Shares.IsPositive())

	// Position's delegator address should have no remaining delegation after full exit.
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().Error(err, "position's delegation should be fully removed after full exit")
}

// TestMsgExitTierWithDelegation_FullExitNearTotalSlash verifies that a full
// exit works when the validator has been slashed to near-zero. The position
// has a very small token value but DelegatedShares still holds shares. The full
// exit should cleanly unbond all shares and delete the position.
func (s *KeeperSuite) TestMsgExitTierWithDelegation_FullExitNearTotalSlash() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Slash 99% — position token value goes very low but shares remain.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
	s.slashValidatorDirect(valAddr, sdkmath.LegacyNewDecWithPrec(99, 2))

	pos, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(pos.IsDelegated())
	s.Require().True(pos.DelegatedShares.IsPositive(), "shares should still exist")

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)
	s.Require().True(tokenValue.IsPositive(), "token value should be small but positive after 99% slash")

	s.advancePastExitDuration()

	// Full exit with the tiny remaining token value.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().NoError(err)
	s.Require().True(resp.FullExit)

	// Position should be deleted.
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)

	// Owner should have a delegation (even if tiny).
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(del.Shares.IsPositive())

	// Position's delegator address should have no remaining delegation after full exit.
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().Error(err, "position's delegation should be fully removed after full exit")
}

// TestMsgExitTierWithDelegation_FullExitSweepsNonBondDenomDust verifies that
// stray coins on the position's delegator account are swept to the owner when
// a full exit deletes the position. Dust shouldn't exist in practice (delegation
// transfer moves shares, not coins; rewards route to the owner), but the
// handler is defensively tolerant.
func (s *KeeperSuite) TestMsgExitTierWithDelegation_FullExitSweepsNonBondDenomDust() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Inject dust directly onto the position's delegator account.
	dustDenom := "dust"
	dustAmount := sdkmath.NewInt(123)
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, posDelAddr,
		sdk.NewCoins(sdk.NewCoin(dustDenom, dustAmount))))

	dustBefore := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, dustDenom)

	tokenValue, err := s.keeper.PositionTokenValue(s.ctx, pos)
	s.Require().NoError(err)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ExitTierWithDelegation(s.ctx, &types.MsgExitTierWithDelegation{
		Owner:      pos.Owner,
		PositionId: pos.Id,
		Amount:     tokenValue,
	})
	s.Require().NoError(err)
	s.Require().True(resp.FullExit, "full exit expected")

	dustAfter := s.app.BankKeeper.GetBalance(s.ctx, ownerAddr, dustDenom)
	s.Require().True(dustAfter.Amount.Equal(dustBefore.Amount.Add(dustAmount)),
		"owner should have received the dust on full exit sweep")

	s.Require().True(s.app.BankKeeper.GetAllBalances(s.ctx, posDelAddr).IsZero(),
		"position's delegator account should be empty after sweep")
}
