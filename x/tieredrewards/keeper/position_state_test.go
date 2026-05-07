package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestLoadPosition_Delegated() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, false)

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().Equal(pos.Id, state.Id)
	s.Require().Equal(pos.Owner, state.Owner)

	s.Require().True(state.IsDelegated())
	s.Require().NotNil(state.Delegation)
	s.Require().Equal(pos.Delegation.ValidatorAddress, state.Delegation.ValidatorAddress)
	s.Require().True(pos.Delegation.Shares.Equal(state.Delegation.Shares))

	amount, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)
	s.Require().Equal(lockAmount.String(), amount.String())
}

func (s *KeeperSuite) TestLoadPosition_Unbonding() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(state.IsDelegated())
	s.Require().Equal(pos.IsDelegated(), state.IsDelegated())
	s.Require().Nil(state.Delegation)

	amount, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)
	s.Require().Equal(s.getPositionAmount(pos).String(), amount.String())
	s.Require().Equal(lockAmount.String(), amount.String())
}

func (s *KeeperSuite) TestLoadPosition_UnbondingComplete() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.completeStakingUnbonding(valAddr, types.GetDelegatorAddress(pos.Id))

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(state.IsDelegated())
	s.Require().Nil(state.Delegation)

	amount, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)
	s.Require().Equal(s.getPositionAmount(pos).String(), amount.String())
	s.Require().Equal(lockAmount.String(), amount.String())
}

func (s *KeeperSuite) TestLoadPosition_NotFound() {
	_, err := s.keeper.GetPositionState(s.ctx, 99999)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
}

func (s *KeeperSuite) TestPositionAmount_DelegationSlashed() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	preSlash, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)

	slashFraction := sdkmath.LegacyNewDecWithPrec(10, 2)
	s.slashValidatorDirect(valAddr, slashFraction)

	state, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	postSlash, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)

	expected := sdkmath.LegacyNewDecFromInt(preSlash).
		Mul(sdkmath.LegacyOneDec().Sub(slashFraction)).
		TruncateInt()
	s.Require().Equal(expected.String(), postSlash.String(),
		"postSlash should equal preSlash × (1 - slashFraction)")
}

func (s *KeeperSuite) TestPositionAmount_PartiallySlashedUnbonding() {
	lockAmount := sdkmath.NewInt(4000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	slashAmount := s.getPositionAmount(pos).QuoRaw(2)
	s.slashUnbondingEntry(types.GetDelegatorAddress(pos.Id), valAddr, slashAmount)

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	amount, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)

	s.Require().Equal(lockAmount.Sub(slashAmount).String(), amount.String(),
		"derived amount during partial-slash unbonding must match the reduced UD balance")
}

func (s *KeeperSuite) TestPositionAmount_IncludesOnlyBondDenom() {
	lockAmount := sdkmath.NewInt(1500)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	delAddr := types.GetDelegatorAddress(pos.Id)
	s.completeStakingUnbonding(valAddr, delAddr)

	// Sprinkle stray dust of a different denom.
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
		sdk.NewCoins(sdk.NewCoin("dust", sdkmath.NewInt(999)))))

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	amount, err := s.keeper.GetPositionAmount(s.ctx, state)
	s.Require().NoError(err)

	s.Require().Equal(lockAmount.String(), amount.String())
}

// TestGetPositionStatesByOwner_SkipsStaleIndexEntries verifies that a
// PositionsByOwner entry pointing to a missing Position (a soft consistency
// bug) is silently skipped rather than failing the whole lookup. The tally
// path relies on this to avoid halting the end-blocker.
func (s *KeeperSuite) TestGetPositionStatesByOwner_SkipsStaleIndexEntries() {
	live := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	stale := s.setupNewTierPosition(sdkmath.NewInt(2000), false)

	owner := sdk.MustAccAddressFromBech32(live.Owner)
	// Both setups use fresh random owners, so force them onto the same owner
	// index entry for this test.
	s.Require().NoError(s.keeper.PositionsByOwner.Set(s.ctx,
		collections.Join(owner, stale.Id)))

	// Delete only the stored Position for the stale entry — the owner index
	// still references its ID.
	s.Require().NoError(s.keeper.Positions.Remove(s.ctx, stale.Id))

	states, err := s.keeper.GetPositionStatesByOwner(s.ctx, owner)
	s.Require().NoError(err, "stale index entry should be skipped, not error")
	s.Require().Len(states, 1, "only the live position should be returned")
	s.Require().Equal(live.Id, states[0].Id)
}
