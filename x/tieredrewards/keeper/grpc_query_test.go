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

func (s *KeeperSuite) TestGRPCQueryTierPosition_NilRequest() {
	srv := keeper.NewQueryServerImpl(s.keeper)
	_, err := srv.TierPosition(s.ctx, nil)
	s.Require().Error(err)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
	s.Require().ErrorContains(err, "empty request")
}

// --- TierPositionsByOwner ---

func (s *KeeperSuite) TestGRPCQueryTierPositionsByOwner() {
	owner := testPositionOwner
	otherOwner := sdk.AccAddress([]byte("query_other_owner___")).String()

	pos1 := newTestPosition(1, owner, 1)
	pos2 := newTestPosition(2, owner, 2)
	pos3 := newTestPosition(3, otherOwner, 1)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos1))
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos2))
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos3))

	resp, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{Owner: owner})
	s.Require().NoError(err)
	s.Require().Len(resp.Positions, 2)
	expectedIDs := map[uint64]struct{}{
		1: {},
		2: {},
	}
	for _, pos := range resp.Positions {
		s.Require().Equal(owner, pos.Owner, "query must only return positions owned by requested owner")
		_, ok := expectedIDs[pos.Id]
		s.Require().True(ok, "unexpected position id %d returned for owner %s", pos.Id, owner)
		delete(expectedIDs, pos.Id)
	}
	s.Require().Empty(expectedIDs, "missing expected positions for owner %s", owner)
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
	s.Require().Len(seen, 5)
	for i := uint64(1); i <= 5; i++ {
		_, ok := seen[i]
		s.Require().True(ok, "missing position id %d after paginating all pages", i)
	}
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

// --- RewardsPoolBalance ---

func (s *KeeperSuite) TestGRPCQueryRewardsPoolBalance_Empty() {
	resp, err := s.queryClient.RewardsPoolBalance(s.ctx.Context(), &types.QueryRewardsPoolBalanceRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.Balance.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryRewardsPoolBalance_WithFunds() {
	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	fundAmount := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(50000)))
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, fundAmount)
	s.Require().NoError(err)

	resp, err := s.queryClient.RewardsPoolBalance(s.ctx.Context(), &types.QueryRewardsPoolBalanceRequest{})
	s.Require().NoError(err)
	s.Require().Equal(fundAmount, resp.Balance)
	s.Require().NotEmpty(resp.Address)
	s.Require().Equal(s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName).String(), resp.Address)
}

// --- EstimatePositionRewards ---

func (s *KeeperSuite) TestGRPCQueryEstimatePositionRewards_NotDelegated() {
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

	resp, err := s.queryClient.EstimatePositionRewards(s.ctx.Context(), &types.QueryEstimatePositionRewardsRequest{PositionId: 0})
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
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	// Set validator commission to 0% so all staking rewards go to delegators.
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Fund bonus pool so bonus rewards can accrue.
	s.fundRewardsPool(sdkmath.NewInt(1000000000), bondDenom)

	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

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

	ownerBalBefore := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	moduleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	moduleBalBefore := s.app.BankKeeper.GetBalance(s.ctx, moduleAddr, bondDenom)
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

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	expectedBonus := s.keeper.CalculateBonusRaw(posBefore, val, tier, s.ctx.BlockTime())
	actualBonus := resp.BonusRewards.AmountOf(bondDenom)
	s.Require().Equal(expectedBonus.String(), actualBonus.String(),
		"bonus rewards should match what is calculated")
	posAfter, err := s.keeper.GetPosition(s.ctx, 0)
	s.Require().NoError(err)
	s.Require().Equal(posBefore, posAfter, "position state must remain unchanged by estimation")

	ownerBalAfter := s.app.BankKeeper.GetBalance(s.ctx, delAddr, bondDenom)
	moduleBalAfter := s.app.BankKeeper.GetBalance(s.ctx, moduleAddr, bondDenom)
	rewardsPoolBalAfter := s.app.BankKeeper.GetBalance(s.ctx, rewardsPoolAddr, bondDenom)
	s.Require().Equal(ownerBalBefore, ownerBalAfter, "owner balance must not change")
	s.Require().Equal(moduleBalBefore, moduleBalAfter, "tier module balance must not change")
	s.Require().Equal(rewardsPoolBalBefore, rewardsPoolBalAfter, "rewards pool balance must not change")
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

func (s *KeeperSuite) TestGRPCQueryTierVotingPower_UnbondingValidatorStillCounts() {
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

	s.jailAndUnbondValidator(valAddr)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "validator should no longer be bonded")

	positions, err := s.keeper.GetDelegatedPositionsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	expected := val.TokensFromShares(positions[0].DelegatedShares)
	s.Require().True(expected.IsPositive())

	s.resetQueryClient()
	resp, err := s.queryClient.TierVotingPower(s.ctx.Context(), &types.QueryTierVotingPowerRequest{Voter: delAddr.String()})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.Equal(expected),
		"delegated position should still count when validator is unbonding; got %s, expected %s", resp.VotingPower, expected)
}

// --- TotalDelegatedVotingPower ---

func (s *KeeperSuite) TestGRPCQueryTotalDelegatedVotingPower_Empty() {
	s.resetQueryClient()

	resp, err := s.queryClient.TotalDelegatedVotingPower(s.ctx.Context(), &types.QueryTotalDelegatedVotingPowerRequest{})
	s.Require().NoError(err)
	s.Require().True(resp.VotingPower.IsZero())
}

func (s *KeeperSuite) TestGRPCQueryTotalDelegatedVotingPower_Delegated() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	lockAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	otherAddr := sdk.AccAddress([]byte("other_delegator______"))
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, otherAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.LockFunds(s.ctx, otherAddr.String(), lockAmount))
	shares, err := s.keeper.Delegate(s.ctx, valAddr, lockAmount)
	s.Require().NoError(err)
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)
	currentRatio, err := s.keeper.GetValidatorRewardRatio(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.keeper.CreatePosition(s.ctx, otherAddr.String(), tier, lockAmount, types.Delegation{
		Validator:           valAddr.String(),
		Shares:              shares,
		BaseRewardsPerShare: currentRatio,
	}, false)
	s.Require().NoError(err)

	s.resetQueryClient()
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
