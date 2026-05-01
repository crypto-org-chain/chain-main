package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) resetQueryClient() {
	queryHelper := baseapp.NewQueryServerTestHelper(s.ctx, s.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, keeper.NewQueryServerImpl(s.app.TieredRewardsKeeper))
	s.queryClient = types.NewQueryClient(queryHelper)
}

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
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), false)

	resp, err := s.queryClient.TierPosition(s.ctx.Context(), &types.QueryTierPositionRequest{PositionId: pos.Id})
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, resp.Position.Id)
	s.Require().Equal(pos.Owner, resp.Position.Owner)
	s.Require().True(resp.Position.Amount.IsPositive(), "amount should be the computed token value for delegated position")
}

func (s *KeeperSuite) TestGRPCQueryTierPosition_NotFound() {
	_, err := s.queryClient.TierPosition(s.ctx.Context(), &types.QueryTierPositionRequest{PositionId: 999})
	s.Require().Error(err)
}

func (s *KeeperSuite) TestGRPCQueryTierPosition_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.TierPosition(s.ctx, nil)
	s.Require().Error(err)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
	s.Require().ErrorContains(err, "empty request")
}

// --- TierPositionsByOwner ---

func (s *KeeperSuite) TestGRPCQueryTierPositionsByOwner() {
	pos1 := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	_ = s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false) // another owner
	owner := pos1.Owner

	resp, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{Owner: owner})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 1)
	for _, pos := range resp.Positions {
		s.Require().Equal(owner, pos.Owner)
		s.Require().True(pos.Amount.IsPositive(), "amount should be computed token value")
	}
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

func (s *KeeperSuite) TestGRPCQueryTierPositionsByOwner_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.TierPositionsByOwner(s.ctx, nil)
	s.Require().Error(err)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
	s.Require().ErrorContains(err, "empty request")
}

// --- AllTierPositions ---

func (s *KeeperSuite) TestGRPCQueryAllTierPositions() {
	for i := 0; i < 5; i++ {
		s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	}

	resp, err := s.queryClient.AllTierPositions(s.ctx.Context(), &types.QueryAllTierPositionsRequest{})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 5)
	for _, pos := range resp.Positions {
		s.Require().True(pos.Amount.IsPositive(), "amount should be computed token value")
	}
}

func (s *KeeperSuite) TestGRPCQueryAllTierPositions_Pagination() {
	for i := 0; i < 5; i++ {
		s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
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

	// Pagination must return each position exactly once: no skips, no duplicates, no overlap between pages.
	seen := make(map[uint64]struct{}, 5)
	for _, p := range resp.Positions {
		_, dup := seen[p.Id]
		s.Require().False(dup, "duplicate position id %d within first page", p.Id)
		seen[p.Id] = struct{}{}
	}
	for _, p := range resp2.Positions {
		_, dup := seen[p.Id]
		s.Require().False(dup, "duplicate position id %d across pages or within second page", p.Id)
		seen[p.Id] = struct{}{}
	}
	s.Require().Len(seen, 5, "pagination must return all 5 positions exactly once")
}

func (s *KeeperSuite) TestGRPCQueryAllTierPositions_Empty() {
	resp, err := s.queryClient.AllTierPositions(s.ctx.Context(), &types.QueryAllTierPositionsRequest{})
	s.Require().NoError(err)
	s.Require().Empty(resp.Positions)
}

func (s *KeeperSuite) TestGRPCQueryAllTierPositions_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.AllTierPositions(s.ctx, nil)
	s.Require().Error(err)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
	s.Require().ErrorContains(err, "empty request")
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

// --- RewardsPoolBalances ---

func (s *KeeperSuite) TestGRPCQueryRewardsPoolBalances_Empty() {
	resp, err := s.queryClient.RewardsPoolBalances(s.ctx.Context(), &types.QueryRewardsPoolBalancesRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.Balances.Empty())
}

func (s *KeeperSuite) TestGRPCQueryRewardsPoolBalances_WithFunds() {
	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	fundAmount := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(50000)))
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, fundAmount)
	s.Require().NoError(err)

	resp, err := s.queryClient.RewardsPoolBalances(s.ctx.Context(), &types.QueryRewardsPoolBalancesRequest{})
	s.Require().NoError(err)
	s.Require().Equal(fundAmount, resp.Balances)
	s.Require().NotEmpty(resp.Address)
	s.Require().Equal(s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName).String(), resp.Address)
}

// --- EstimatePositionRewards ---

func (s *KeeperSuite) TestGRPCQueryEstimatePositionRewards_NotDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	s.advancePastExitDuration()
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: pos.Id})
	s.Require().NoError(err)
	s.resetQueryClient()

	resp, err := s.queryClient.EstimatePositionRewards(s.ctx.Context(), &types.QueryEstimatePositionRewardsRequest{PositionId: pos.Id})
	s.Require().NoError(err)
	s.Require().True(resp.BaseRewards.IsZero())
	s.Require().True(resp.BonusRewards.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryEstimatePositionRewards_NotFound() {
	_, err := s.queryClient.EstimatePositionRewards(s.ctx.Context(), &types.QueryEstimatePositionRewardsRequest{PositionId: 999})
	s.Require().Error(err)
}

func (s *KeeperSuite) TestGRPCQueryEstimatePositionRewards_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.EstimatePositionRewards(s.ctx, nil)
	s.Require().Error(err)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
	s.Require().ErrorContains(err, "empty request")
}

func (s *KeeperSuite) TestGRPCQueryEstimatePositionRewards_DelegatedWithBaseAndBonus() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	_, bondDenom := s.getStakingData()

	posBefore, err := s.keeper.GetPosition(s.ctx, uint64(0))
	s.Require().NoError(err)

	// Advance one block so the delegation's starting period in x/distribution
	// is finalized before rewards are allocated.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	// Allocate staking rewards so base rewards accrue.
	rewardAmount := sdkmath.NewInt(10000000)
	s.allocateRewardsToValidator(valAddr, rewardAmount, bondDenom)

	// Advance 30 days to accrue bonus.
	advanceDuration := 30 * 24 * time.Hour
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(advanceDuration))

	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)

	ownerBalBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	rewardsPoolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	rewardsPoolBalBefore := s.app.BankKeeper.GetBalance(s.ctx, rewardsPoolAddr, bondDenom)

	// Call through a cache-wrapped context, matching the ABCI query path
	// (baseapp.CreateQueryContext uses CacheMultiStoreWithVersion) so that
	// state mutations from claimRewardsForPosition are discarded.
	cacheCtx, _ := s.ctx.CacheContext()
	srv := keeper.NewQueryServerImpl(s.keeper)
	resp, err := srv.EstimatePositionRewards(cacheCtx, &types.QueryEstimatePositionRewardsRequest{PositionId: 0})
	s.Require().NoError(err)

	// Base rewards: the genesis validator has one delegation of DefaultPowerReduction.
	// LockTier added a second equal delegation from the tier module account.
	// With 0% commission, the tier module's delegation gets half the rewards.
	// base = rewardAmount / 2
	expectedBase := rewardAmount.Quo(sdkmath.NewInt(2))
	actualBase := resp.BaseRewards.AmountOf(bondDenom)
	s.Require().Equal(expectedBase.String(), actualBase.String(),
		"base rewards should equal half the allocated rewards")

	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	s.Require().NoError(err)

	tokensPerShare, err := s.keeper.GetTokensPerShare(s.ctx, valAddr)
	s.Require().NoError(err)

	expectedBonus := s.keeper.ComputeSegmentBonus(&posBefore, tier, posBefore.LastBonusAccrual, s.ctx.BlockTime(), tokensPerShare)
	actualBonus := resp.BonusRewards.AmountOf(bondDenom)
	s.Require().Equal(expectedBonus.String(), actualBonus.String(),
		"bonus rewards should match what is calculated")
	posAfter, err := s.keeper.GetPosition(s.ctx, 0)
	s.Require().NoError(err)
	s.Require().Equal(posBefore, posAfter, "position state must remain unchanged by estimation")

	ownerBalAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	rewardsPoolBalAfter := s.app.BankKeeper.GetBalance(s.ctx, rewardsPoolAddr, bondDenom)
	s.Require().Equal(ownerBalBefore, ownerBalAfter, "owner balance must not change")
	s.Require().Equal(rewardsPoolBalBefore, rewardsPoolBalAfter, "rewards pool balance must not change")
}

// --- VotingPowerByOwner ---

func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_NoDelegated() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	s.advancePastExitDuration()
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: pos.Id})
	s.Require().NoError(err)
	s.resetQueryClient()

	resp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_Delegated() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	s.resetQueryClient()

	resp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)))
}

func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_NoPositions() {
	s.resetQueryClient()
	addr := sdk.AccAddress([]byte("no_positions________")).String()

	resp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: addr})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero())
}

// TestGRPCQueryVotingPowerByOwner_NilRequest verifies the nil guard added to the
// VotingPowerByOwner handler returns InvalidArgument instead of panicking.
func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.VotingPowerByOwner(s.ctx, nil)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "empty request")
}

// TestGRPCQueryVotingPowerByOwner_InvalidAddress verifies that a malformed bech32
// address returns an error rather than panicking or returning zero power silently.
func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_InvalidAddress() {
	_, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: "not-a-valid-address"})
	s.Require().Error(err)
}

// TestGRPCQueryVotingPowerByOwner_ExitingPosition verifies that a position with a
// triggered exit still contributes to voting power.
func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_ExitingPosition() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	s.resetQueryClient()

	// Before triggering exit: should have positive voting power.
	resp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"active delegated position should contribute voting power; got %s", resp.VotingPower)

	// Trigger exit.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      delAddr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)
	s.resetQueryClient()

	// After triggering exit: still-delegated positions continue to contribute voting power.
	resp, err = s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount)),
		"exiting but still delegated position should report voting power; got %s", resp.VotingPower)
}

func (s *KeeperSuite) TestGRPCQueryVotingPowerByOwner_UnbondingValidatorNotCounted() {
	lockAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(lockAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)

	s.jailAndUnbondValidator(valAddr)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "validator should no longer be bonded")

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	s.resetQueryClient()
	resp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero(),
		"delegated position on unbonding validator should not count; got %s", resp.VotingPower)
}

// --- TotalDelegatedVotingPower ---

func (s *KeeperSuite) TestGRPCQueryTotalDelegatedVotingPower_Empty() {
	s.resetQueryClient()

	resp, err := s.queryClient.TotalDelegatedVotingPower(s.ctx.Context(), &types.QueryTotalDelegatedVotingPowerRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryTotalDelegatedVotingPower_Delegated() {
	lockAmount := sdkmath.NewInt(5000)
	s.setupNewTierPosition(lockAmount, false)
	s.setupNewTierPosition(lockAmount, false)

	resp, err := s.queryClient.TotalDelegatedVotingPower(s.ctx.Context(), &types.QueryTotalDelegatedVotingPowerRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(sdkmath.LegacyNewDecFromInt(lockAmount.MulRaw(2))))
}

func (s *KeeperSuite) TestGRPCQueryTotalDelegatedVotingPower_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.TotalDelegatedVotingPower(s.ctx, nil)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "empty request")
}

// ---------------------------------------------------------------------------
// Raw position queries
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestGRPCQueryRawTierPosition() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)

	resp, err := s.queryClient.RawTierPosition(s.ctx.Context(), &types.QueryRawTierPositionRequest{PositionId: pos.Id})
	s.Require().NoError(err)
	s.Require().Equal(pos.Id, resp.Position.Id)
	s.Require().Equal(pos.Owner, resp.Position.Owner)
	s.Require().True(resp.Position.Amount.IsZero(), "raw delegated position amount should be zero")
}

func (s *KeeperSuite) TestGRPCQueryRawTierPositionsByOwner() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)

	resp, err := s.queryClient.RawTierPositionsByOwner(s.ctx.Context(), &types.QueryRawTierPositionsByOwnerRequest{Owner: pos.Owner})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 1)
	s.Require().Equal(pos.Id, resp.Positions[0].Id)
	s.Require().True(resp.Positions[0].Amount.IsZero(), "raw delegated position amount should be zero")
}

func (s *KeeperSuite) TestGRPCQueryRawAllTierPositions() {
	for i := 0; i < 3; i++ {
		s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	}

	resp, err := s.queryClient.RawAllTierPositions(s.ctx.Context(), &types.QueryRawAllTierPositionsRequest{})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 3)
}

// ---------------------------------------------------------------------------
// Validator data
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestGRPCQueryValidatorData() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Validator)
	_, bondDenom := s.getStakingData()

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(100_000), bondDenom)

	// Record a slash event.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	err := s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, sdkmath.LegacyNewDecWithPrec(1, 2))
	s.Require().NoError(err)

	resp, err := s.queryClient.ValidatorData(s.ctx.Context(), &types.QueryValidatorDataRequest{Validator: valAddr.String()})
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), resp.PositionCount)
	s.Require().Len(resp.Events, 1)
	s.Require().Equal(types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH, resp.Events[0].EventType)
	s.Require().Equal(uint64(1), resp.EventCurrentSeq)
}

func (s *KeeperSuite) TestGRPCQueryValidatorData_Empty() {
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	resp, err := s.queryClient.ValidatorData(s.ctx.Context(), &types.QueryValidatorDataRequest{Validator: valAddr.String()})
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), resp.PositionCount)
	s.Require().Empty(resp.Events)
	s.Require().Equal(uint64(0), resp.EventCurrentSeq)
}

// ---------------------------------------------------------------------------
// Position mappings
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestGRPCQueryPositionMappings_AfterUndelegate() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), true)

	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(10_000_000), bondDenom)
	s.advancePastExitDuration()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	resp, err := s.queryClient.PositionMappings(s.ctx.Context(), &types.QueryPositionMappingsRequest{PositionId: pos.Id})
	s.Require().NoError(err)
	s.Require().Len(resp.UnbondingIds, 1, "should have 1 unbonding mapping")
	s.Require().Empty(resp.RedelegationIds)
}

func (s *KeeperSuite) TestGRPCQueryPositionMappings_AfterRedelegate() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)
	dstValAddr, _ := s.createSecondValidator()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	resp, err := s.queryClient.PositionMappings(s.ctx.Context(), &types.QueryPositionMappingsRequest{PositionId: pos.Id})
	s.Require().NoError(err)
	s.Require().Empty(resp.UnbondingIds)
	s.Require().Len(resp.RedelegationIds, 1, "should have 1 redelegation mapping")
}

func (s *KeeperSuite) TestGRPCQueryPositionMappings_Empty() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(sdk.DefaultPowerReduction.Int64()), false)

	resp, err := s.queryClient.PositionMappings(s.ctx.Context(), &types.QueryPositionMappingsRequest{PositionId: pos.Id})
	s.Require().NoError(err)
	s.Require().Empty(resp.UnbondingIds)
	s.Require().Empty(resp.RedelegationIds)
}
