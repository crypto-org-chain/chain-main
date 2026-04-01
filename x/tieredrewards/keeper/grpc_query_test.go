package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// --- Params ---

func (s *KeeperSuite) TestGRPCQueryParams() {
	customParams := types.NewParams(
		sdkmath.LegacyNewDecWithPrec(3, 2),
	)
	s.keeper.InitGenesis(s.ctx, &types.GenesisState{Params: customParams})

	resp, err := s.queryClient.Params(s.ctx.Context(), &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().True(customParams.TargetBaseRewardsRate.Equal(resp.Params.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestGRPCQueryParams_Default() {
	defaultGenesis := types.DefaultGenesisState()
	s.keeper.InitGenesis(s.ctx, defaultGenesis)

	resp, err := s.queryClient.Params(s.ctx.Context(), &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.Params.TargetBaseRewardsRate.IsZero())
}

// --- TierPosition ---

func (s *KeeperSuite) TestGRPCQueryTierPosition() {
	pos := newTestPosition(1, testPositionOwner, 1)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))

	resp, err := s.queryClient.TierPosition(s.ctx.Context(), &types.QueryTierPositionRequest{PositionId: 1})
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), resp.Position.Id)
	s.Require().Equal(testPositionOwner, resp.Position.Owner)
	s.Require().True(pos.Amount.Equal(resp.Position.Amount))
}

func (s *KeeperSuite) TestGRPCQueryTierPosition_NotFound() {
	_, err := s.queryClient.TierPosition(s.ctx.Context(), &types.QueryTierPositionRequest{PositionId: 999})
	s.Require().Error(err)
}

// --- TierPositionsByOwner ---

func (s *KeeperSuite) TestGRPCQueryTierPositionsByOwner() {
	owner := testPositionOwner
	pos1 := newTestPosition(1, owner, 1)
	pos2 := newTestPosition(2, owner, 2)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos1))
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos2))

	resp, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{Owner: owner})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 2)
}

func (s *KeeperSuite) TestGRPCQueryTierPositionsByOwner_Empty() {
	otherOwner := sdk.AccAddress([]byte("other_owner_________")).String()

	resp, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{Owner: otherOwner})
	s.Require().NoError(err)
	s.Require().Empty(resp.Positions)
}

func (s *KeeperSuite) TestGRPCQueryTierPositionsByOwner_InvalidAddress() {
	_, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{Owner: "invalid"})
	s.Require().Error(err)
}

// --- AllTierPositions ---

func (s *KeeperSuite) TestGRPCQueryAllTierPositions() {
	for i := uint64(1); i <= 5; i++ {
		pos := newTestPosition(i, testPositionOwner, 1)
		s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))
	}

	resp, err := s.queryClient.AllTierPositions(s.ctx.Context(), &types.QueryAllTierPositionsRequest{})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 5)
}

func (s *KeeperSuite) TestGRPCQueryAllTierPositions_Pagination() {
	for i := uint64(1); i <= 5; i++ {
		pos := newTestPosition(i, testPositionOwner, 1)
		s.Require().NoError(s.keeper.SetPosition(s.ctx, pos))
	}

	resp, err := s.queryClient.AllTierPositions(s.ctx.Context(), &types.QueryAllTierPositionsRequest{
		Pagination: &query.PageRequest{Limit: 2},
	})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 2)
	s.Require().NotNil(resp.Pagination)
	s.Require().NotEmpty(resp.Pagination.NextKey)

	resp2, err := s.queryClient.AllTierPositions(s.ctx.Context(), &types.QueryAllTierPositionsRequest{
		Pagination: &query.PageRequest{Key: resp.Pagination.NextKey, Limit: 10},
	})
	s.Require().NoError(err)
	s.Require().Len(resp2.Positions, 3)
}

func (s *KeeperSuite) TestGRPCQueryAllTierPositions_Empty() {
	resp, err := s.queryClient.AllTierPositions(s.ctx.Context(), &types.QueryAllTierPositionsRequest{})
	s.Require().NoError(err)
	s.Require().Empty(resp.Positions)
}

// --- Tiers ---

func (s *KeeperSuite) TestGRPCQueryTiers() {
	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(1)))
	s.Require().NoError(s.keeper.SetTier(s.ctx, newTestTier(2)))

	resp, err := s.queryClient.Tiers(s.ctx.Context(), &types.QueryTiersRequest{})
	s.Require().NoError(err)
	s.Require().Len(resp.Tiers, 2)
	s.Require().Equal(uint32(1), resp.Tiers[0].Id)
	s.Require().Equal(uint32(2), resp.Tiers[1].Id)
}

func (s *KeeperSuite) TestGRPCQueryTiers_Empty() {
	resp, err := s.queryClient.Tiers(s.ctx.Context(), &types.QueryTiersRequest{})
	s.Require().NoError(err)
	s.Require().Empty(resp.Tiers)
}

// --- TierPoolBalance ---

func (s *KeeperSuite) TestGRPCQueryTierPoolBalance_Empty() {
	resp, err := s.queryClient.TierPoolBalance(s.ctx.Context(), &types.QueryTierPoolBalanceRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.Balance.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryTierPoolBalance_WithFunds() {
	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	fundAmount := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(50000)))
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, fundAmount)
	s.Require().NoError(err)

	resp, err := s.queryClient.TierPoolBalance(s.ctx.Context(), &types.QueryTierPoolBalanceRequest{})
	s.Require().NoError(err)
	s.Require().Equal(fundAmount, resp.Balance)
}

// --- EstimateTierRewards ---

func (s *KeeperSuite) TestGRPCQueryEstimateTierRewards_NotDelegated() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock WITH delegation and immediate exit, then undelegate to get an undelegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(5000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)
	s.resetQueryClient()

	resp, err := s.queryClient.EstimateTierRewards(s.ctx.Context(), &types.QueryEstimateTierRewardsRequest{PositionId: 0})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero())
	s.Require().True(resp.BonusRewards.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryEstimateTierRewards_NotFound() {
	_, err := s.queryClient.EstimateTierRewards(s.ctx.Context(), &types.QueryEstimateTierRewardsRequest{PositionId: 999})
	s.Require().Error(err)
}

func (s *KeeperSuite) TestGRPCQueryEstimateTierRewards_DelegatedWithBonus() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	lockAmount := sdkmath.NewInt(10000)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, delAddr.String(), lockAmount))

	shares, err := s.keeper.Delegate(s.ctx, valAddr, lockAmount)
	s.Require().NoError(err)

	currentRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)

	delegation := types.Delegation{
		Validator:           valAddr.String(),
		Shares:              shares,
		BaseRewardsPerShare: currentRatio,
	}

	pos, err := s.keeper.CreatePosition(s.ctx, delAddr.String(), tier, lockAmount, delegation, false)
	s.Require().NoError(err)

	// Advance block time by 30 days to accrue bonus
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))
	s.resetQueryClient()

	resp, err := s.queryClient.EstimateTierRewards(s.ctx.Context(), &types.QueryEstimateTierRewardsRequest{PositionId: pos.Id})
	s.Require().NoError(err)

	// Bonus should be positive (10000 * 0.04 * 30days/365.25days > 0)
	hasBondDenom := false
	for _, c := range resp.BonusRewards {
		if c.Denom == bondDenom {
			hasBondDenom = true
			s.Require().True(c.Amount.IsPositive(), "bonus reward should be positive, got %s", c.Amount)
		}
	}
	s.Require().True(hasBondDenom, "bonus rewards should contain bond denom")
}

// --- TierVotingPower ---

func (s *KeeperSuite) TestGRPCQueryTierVotingPower_NoDelegated() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Lock WITH delegation and immediate exit, then undelegate to get an undelegated position
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 sdkmath.NewInt(5000),
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(1000000), bondDenom)
	s.advancePastExitDuration()
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)
	s.resetQueryClient()

	resp, err := s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryTierVotingPower_Delegated() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)
	s.resetQueryClient()

	resp, err := s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)))
}

func (s *KeeperSuite) TestGRPCQueryTierVotingPower_NoPositions() {
	s.resetQueryClient()
	addr := sdk.AccAddress([]byte("no_positions________")).String()

	resp, err := s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: addr})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero())
}

// TestGRPCQueryTierVotingPower_NilRequest verifies the nil guard added to the
// TierVotingPower handler returns InvalidArgument instead of panicking.
func (s *KeeperSuite) TestGRPCQueryTierVotingPower_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.TierVotingPower(s.ctx, nil)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "empty request")
}

// TestGRPCQueryTierVotingPower_InvalidAddress verifies that a malformed bech32
// address returns an error rather than panicking or returning zero power silently.
func (s *KeeperSuite) TestGRPCQueryTierVotingPower_InvalidAddress() {
	_, err := s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: "not-a-valid-address"})
	s.Require().Error(err)
}

// TestGRPCQueryTierVotingPower_ExitingPosition verifies that a position with a
// triggered exit still contributes to voting power per ADR-006 §8.5.
func (s *KeeperSuite) TestGRPCQueryTierVotingPower_ExitingPosition() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)
	s.resetQueryClient()

	// Before triggering exit: should have positive voting power.
	resp, err := s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"active delegated position should contribute voting power; got %s", resp.VotingPower)

	// Trigger exit.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: 0,
	})
	s.Require().NoError(err)
	s.resetQueryClient()

	// After triggering exit: per ADR-006 §8.5, still-delegated positions
	// continue to contribute voting power.
	resp, err = s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exiting but still delegated position should report voting power; got %s", resp.VotingPower)
}
