package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestLoadPosition_Delegated() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, false)

	state, err := s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().Equal(pos.Id, state.Id)
	s.Require().Equal(pos.Owner, state.Owner)

	s.Require().True(state.IsDelegated())
	s.Require().NotNil(state.Delegation)
	s.Require().Equal(pos.Validator, state.Delegation.ValidatorAddress)
	s.Require().True(pos.DelegatedShares.Equal(state.Delegation.Shares))

	amount, err := s.keeper.PositionAmount(s.ctx, state)
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

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	state, err := s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(state.IsDelegated())
	s.Require().Equal(pos.IsDelegated(), state.IsDelegated())
	s.Require().Nil(state.Delegation)

	amount, err := s.keeper.PositionAmount(s.ctx, state)
	s.Require().NoError(err)
	s.Require().Equal(pos.Amount.String(), amount.String())
	s.Require().Equal(lockAmount.String(), amount.String())
}

func (s *KeeperSuite) TestLoadPosition_UnbondingComplete() {
	lockAmount := sdkmath.NewInt(1000)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
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

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	state, err := s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(state.IsDelegated())
	s.Require().Nil(state.Delegation)

	amount, err := s.keeper.PositionAmount(s.ctx, state)
	s.Require().NoError(err)
	s.Require().Equal(pos.Amount.String(), amount.String())
	s.Require().Equal(lockAmount.String(), amount.String())
}

func (s *KeeperSuite) TestLoadPosition_NotFound() {
	_, err := s.keeper.LoadPosition(s.ctx, 99999)
	s.Require().ErrorIs(err, types.ErrPositionNotFound)
}

func (s *KeeperSuite) TestPositionAmount_DelegationSlashed() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	state, err := s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	preSlash, err := s.keeper.PositionAmount(s.ctx, state)
	s.Require().NoError(err)

	slashFraction := sdkmath.LegacyNewDecWithPrec(10, 2)
	s.slashValidatorDirect(valAddr, slashFraction)

	state, err = s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	postSlash, err := s.keeper.PositionAmount(s.ctx, state)
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
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	undelResp, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	slashAmount := pos.Amount.QuoRaw(2)
	s.slashUnbondingEntry(types.GetDelegatorAddress(pos.Id), valAddr, undelResp.UnbondingId, slashAmount)

	state, err := s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	amount, err := s.keeper.PositionAmount(s.ctx, state)
	s.Require().NoError(err)

	s.Require().Equal(lockAmount.Sub(slashAmount).String(), amount.String(),
		"derived amount during partial-slash unbonding must match the reduced UD balance")
}

func (s *KeeperSuite) TestPositionAmount_WithdrawableIncludesOnlyBondDenom() {
	lockAmount := sdkmath.NewInt(1500)
	pos := s.setupNewTierPosition(lockAmount, true)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
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

	state, err := s.keeper.LoadPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	amount, err := s.keeper.PositionAmount(s.ctx, state)
	s.Require().NoError(err)

	s.Require().Equal(lockAmount.String(), amount.String(),
		"positionAmount must ignore non-bondDenom dust")
}
