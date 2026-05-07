package keeper_test

import (
	"context"
	"errors"
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections"
	addresscodec "cosmossdk.io/core/address"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ---------------------------------------------------------------------------
// Mock + standalone tests
// ---------------------------------------------------------------------------

// mockTierVotingPower implements TierVotingPowerProvider for unit tests.
// Set getErr to simulate errors from the keeper.
type mockTierVotingPower struct {
	positions map[string][]types.PositionState
	getErr    error
}

func (m mockTierVotingPower) GetPositionStatesByOwner(_ context.Context, voter sdk.AccAddress) ([]types.PositionState, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.positions[voter.String()], nil
}

var _ keeper.TierVotingPowerProvider = mockTierVotingPower{}

type rawStringCodec struct{}

func (rawStringCodec) StringToBytes(text string) ([]byte, error) {
	return []byte(text), nil
}

func (rawStringCodec) BytesToString(bz []byte) (string, error) {
	return string(bz), nil
}

type mockGovTallyStakingKeeper struct{}

func (mockGovTallyStakingKeeper) ValidatorAddressCodec() addresscodec.Codec {
	return rawStringCodec{}
}

func (mockGovTallyStakingKeeper) IterateDelegations(_ context.Context, _ sdk.AccAddress, _ func(int64, stakingtypes.DelegationI) bool) error {
	return nil
}

func (mockGovTallyStakingKeeper) GetValidator(_ context.Context, _ sdk.ValAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
}

type mockGovTallyAccountKeeper struct{}

func (mockGovTallyAccountKeeper) AddressCodec() addresscodec.Codec {
	return rawStringCodec{}
}

func TestNewCalculateVoteResultsAndVotingPowerFn_NotNil(t *testing.T) {
	mock := mockTierVotingPower{positions: map[string][]types.PositionState{}}
	fn := keeper.NewCustomTallyTierVotesFn(mock, nil, nil)
	require.NotNil(t, fn, "should return a non-nil tally function")
}

func (s *KeeperSuite) TestTierVotingPowerProvider_KeeperSatisfied() {
	var _ keeper.TierVotingPowerProvider = s.keeper
}

// ---------------------------------------------------------------------------
// Integration test helpers
// ---------------------------------------------------------------------------

const testProposalID uint64 = 999

func yesVoteOpts() []*v1.WeightedVoteOption {
	return []*v1.WeightedVoteOption{{Option: v1.OptionYes, Weight: "1.000000000000000000"}}
}

func noVoteOpts() []*v1.WeightedVoteOption {
	return []*v1.WeightedVoteOption{{Option: v1.OptionNo, Weight: "1.000000000000000000"}}
}

// buildValidatorsMap replicates gov's getCurrentValidators: bonded validators
// with zero deductions and no vote.
func (s *KeeperSuite) buildValidatorsMap() map[string]v1.ValidatorGovInfo {
	validators := make(map[string]v1.ValidatorGovInfo)
	err := s.app.StakingKeeper.IterateBondedValidatorsByPower(s.ctx,
		func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
			valBz, err := s.app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
			if err != nil {
				return false
			}
			validators[val.GetOperator()] = v1.NewValidatorGovInfo(
				valBz,
				val.GetBondedTokens(),
				val.GetDelegatorShares(),
				sdkmath.LegacyZeroDec(),
				v1.WeightedVoteOptions{},
			)
			return false
		})
	s.Require().NoError(err)
	return validators
}

// insertVote writes a vote directly into the gov Votes store.
func (s *KeeperSuite) insertVote(proposalID uint64, voter sdk.AccAddress, opts []*v1.WeightedVoteOption) {
	s.T().Helper()
	vote := v1.Vote{
		ProposalId: proposalID,
		Voter:      voter.String(),
		Options:    opts,
	}
	err := s.app.GovKeeper.Votes.Set(s.ctx, collections.Join(proposalID, voter), vote)
	s.Require().NoError(err)
}

// callCustomTally builds and invokes the custom tally function.
func (s *KeeperSuite) callCustomTally(proposalID uint64, validators map[string]v1.ValidatorGovInfo) (
	sdkmath.LegacyDec, map[v1.VoteOption]sdkmath.LegacyDec,
) {
	s.T().Helper()
	tallyFn := keeper.NewCustomTallyTierVotesFn(
		s.keeper, s.app.StakingKeeper, s.app.AccountKeeper,
	)
	proposal := v1.Proposal{Id: proposalID}
	totalPower, results, err := tallyFn(s.ctx, s.app.GovKeeper, proposal, validators)
	s.Require().NoError(err)
	return totalPower, results
}

// stakingPowerFor computes the staking voting power for delAddr on valAddr
// using the given validators map (same formula the tally uses).
func (s *KeeperSuite) stakingPowerFor(delAddr sdk.AccAddress, valAddr sdk.ValAddress, validators map[string]v1.ValidatorGovInfo) sdkmath.LegacyDec {
	s.T().Helper()
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	valInfo, ok := validators[valAddr.String()]
	if !ok {
		return sdkmath.LegacyZeroDec()
	}
	return del.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)
}

// tierPowerFor computes the exact tier voting power for an address from its
// DelegatedShares using the validators map (same formula used by the tally).
func (s *KeeperSuite) tierPowerFor(owner sdk.AccAddress, validators map[string]v1.ValidatorGovInfo) sdkmath.LegacyDec {
	s.T().Helper()
	positions, err := s.keeper.GetPositionStatesByOwner(s.ctx, owner)
	s.Require().NoError(err)

	total := sdkmath.LegacyZeroDec()
	for _, pos := range positions {
		val, ok := validators[pos.Delegation.ValidatorAddress]
		if !ok || val.DelegatorShares.IsZero() {
			continue
		}
		total = total.Add(pos.Delegation.Shares.MulInt(val.BondedTokens).Quo(val.DelegatorShares))
	}
	return total
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

// TestQueryAndGovTallyWiring_TierPowerConsistency verifies query-layer voting
// power contracts and gov custom tally wiring stay consistent on the same
// state.
func (s *KeeperSuite) TestQueryAndGovTallyWiring_TierPowerConsistency() {
	// Get a baseline total power before creating any tier positions.
	preTotalPowerResp, err := s.queryClient.TotalDelegatedVotingPower(
		s.ctx.Context(),
		&types.QueryTotalDelegatedVotingPowerRequest{},
	)
	s.Require().NoError(err)

	yesAmount := sdkmath.NewInt(7000)
	noAmount := sdkmath.NewInt(4000)
	yesPos := s.setupNewTierPosition(yesAmount, false)
	noPos := s.setupNewTierPosition(noAmount, false)
	yesVoter := sdk.MustAccAddressFromBech32(yesPos.Owner)
	noVoter := sdk.MustAccAddressFromBech32(noPos.Owner)
	valAddr := sdk.MustValAddressFromBech32(yesPos.Delegation.ValidatorAddress)

	// Query contract: each owner query returns only the caller's positions.
	yesOwnerResp, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{
		Owner: yesVoter.String(),
	})
	s.Require().NoError(err)
	s.Require().Len(yesOwnerResp.Positions, 1)
	s.Require().Equal(yesVoter.String(), yesOwnerResp.Positions[0].Owner)
	s.Require().Equal(valAddr.String(), yesOwnerResp.Positions[0].Validator)

	noOwnerResp, err := s.queryClient.TierPositionsByOwner(s.ctx.Context(), &types.QueryTierPositionsByOwnerRequest{
		Owner: noVoter.String(),
	})
	s.Require().NoError(err)
	s.Require().Len(noOwnerResp.Positions, 1)
	s.Require().Equal(noVoter.String(), noOwnerResp.Positions[0].Owner)
	s.Require().Equal(valAddr.String(), noOwnerResp.Positions[0].Validator)

	// Independent oracle (non-query): expected power from position shares.
	validators := s.buildValidatorsMap()
	expectedYesPower := s.tierPowerFor(yesVoter, validators)
	expectedNoPower := s.tierPowerFor(noVoter, validators)
	expectedAddedPower := expectedYesPower.Add(expectedNoPower)
	s.Require().True(expectedYesPower.IsPositive())
	s.Require().True(expectedNoPower.IsPositive())

	yesPowerResp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: yesVoter.String()})
	s.Require().NoError(err)
	s.Require().True(yesPowerResp.VotingPower.Equal(expectedYesPower),
		"yes voter query power should match independent expected power; got %s, want %s",
		yesPowerResp.VotingPower, expectedYesPower)

	noPowerResp, err := s.queryClient.VotingPowerByOwner(s.ctx.Context(), &types.QueryVotingPowerByOwnerRequest{Owner: noVoter.String()})
	s.Require().NoError(err)
	s.Require().True(noPowerResp.VotingPower.Equal(expectedNoPower),
		"no voter query power should match independent expected power; got %s, want %s",
		noPowerResp.VotingPower, expectedNoPower)

	totalPowerResp, err := s.queryClient.TotalDelegatedVotingPower(
		s.ctx.Context(),
		&types.QueryTotalDelegatedVotingPowerRequest{},
	)
	s.Require().NoError(err)
	addedPower := totalPowerResp.VotingPower.Sub(preTotalPowerResp.VotingPower)
	s.Require().True(addedPower.Equal(expectedAddedPower),
		"total delegated voting power delta should equal added voters' power; got %s, want %s",
		addedPower, expectedAddedPower)

	// Gov tally wiring should consume the same underlying tier power values.
	s.insertVote(testProposalID, yesVoter, yesVoteOpts())
	s.insertVote(testProposalID, noVoter, noVoteOpts())

	tallyTotal, results := s.callCustomTally(testProposalID, validators)
	s.Require().True(tallyTotal.Equal(expectedAddedPower),
		"tally total should equal independently expected tier power; got %s, want %s",
		tallyTotal, expectedAddedPower)
	s.Require().True(results[v1.OptionYes].Equal(expectedYesPower),
		"yes tally should equal independent expected yes power; got %s, want %s",
		results[v1.OptionYes], expectedYesPower)
	s.Require().True(results[v1.OptionNo].Equal(expectedNoPower),
		"no tally should equal independent expected no power; got %s, want %s",
		results[v1.OptionNo], expectedNoPower)
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

// A voter with NO staking delegation but WITH a delegated tier position.
// All voting power comes from tier.
func (s *KeeperSuite) TestCustomTally_TierOnlyVoter() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(8000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()

	expected := s.tierPowerFor(delAddr, validators)
	s.Require().True(expected.IsPositive(), "tier power should be positive")

	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(expected),
		"voter with no staking should have power = tier power; got %s, want %s", totalPower, expected)
	s.Require().True(results[v1.OptionYes].Equal(expected))
	s.Require().True(results[v1.OptionNo].IsZero())
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

// A voter with BOTH staking delegation and a delegated tier position.
// Total = staking power + tier power.
func (s *KeeperSuite) TestCustomTally_StakingPlusTier() {
	pos := s.setupNewTierPositionWithDelegator(sdkmath.NewInt(5000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()

	expectedStaking := s.stakingPowerFor(delAddr, valAddr, validators)
	s.Require().True(expectedStaking.IsPositive(), "delegator should have staking power")
	expectedTier := s.tierPowerFor(delAddr, validators)
	expectedTotal := expectedStaking.Add(expectedTier)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(expectedTotal),
		"total should be staking + tier; got %s, want %s", totalPower, expectedTotal)
	s.Require().True(results[v1.OptionYes].Equal(expectedTotal))
}

// Weighted vote (60 % Yes / 40 % No) splits owner's voting power
func (s *KeeperSuite) TestCustomTally_WeightedVoteSplitsPower() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	opts := []*v1.WeightedVoteOption{
		{Option: v1.OptionYes, Weight: "0.600000000000000000"},
		{Option: v1.OptionNo, Weight: "0.400000000000000000"},
	}
	s.insertVote(testProposalID, delAddr, opts)
	validators := s.buildValidatorsMap()

	tierPower := s.tierPowerFor(delAddr, validators)
	s.Require().True(tierPower.IsPositive())

	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(tierPower),
		"total should equal tier power; got %s, want %s", totalPower, tierPower)

	expectedYes := tierPower.Mul(sdkmath.LegacyNewDecWithPrec(6, 1)) // 60 %
	expectedNo := tierPower.Mul(sdkmath.LegacyNewDecWithPrec(4, 1))  // 40 %
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes should be 60%% of tier; got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(expectedNo),
		"No should be 40%% of tier; got %s, want %s", results[v1.OptionNo], expectedNo)
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

func (s *KeeperSuite) TestCustomTally_InvalidVoteWeight_ReturnsError() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	s.insertVote(testProposalID, delAddr, []*v1.WeightedVoteOption{
		{Option: v1.OptionYes, Weight: "not-a-decimal"},
	})

	tallyFn := keeper.NewCustomTallyTierVotesFn(
		s.keeper, s.app.StakingKeeper, s.app.AccountKeeper,
	)
	proposal := v1.Proposal{Id: testProposalID}
	validators := s.buildValidatorsMap()

	_, _, err := tallyFn(s.ctx, s.app.GovKeeper, proposal, validators)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "invalid vote weight")

	hasVote, hasErr := s.app.GovKeeper.Votes.Has(s.ctx, collections.Join(testProposalID, delAddr))
	s.Require().NoError(hasErr)
	s.Require().True(hasVote, "failed tally should not remove the original vote")
}

func (s *KeeperSuite) TestCustomTally_InvalidValidatorVoteWeight_PreservesVote() {
	const validatorVoter = "validator-voter"

	voterAddr := sdk.AccAddress([]byte(validatorVoter))
	err := s.app.GovKeeper.Votes.Set(s.ctx, collections.Join(testProposalID, voterAddr), v1.Vote{
		ProposalId: testProposalID,
		Voter:      validatorVoter,
		Options: []*v1.WeightedVoteOption{
			{Option: v1.OptionNo, Weight: "not-a-decimal"},
		},
	})
	s.Require().NoError(err)

	validators := map[string]v1.ValidatorGovInfo{
		validatorVoter: v1.NewValidatorGovInfo(
			[]byte(validatorVoter),
			sdkmath.NewInt(100),
			sdkmath.LegacyNewDec(100),
			sdkmath.LegacyZeroDec(),
			nil,
		),
	}

	tallyFn := keeper.NewCustomTallyTierVotesFn(
		mockTierVotingPower{positions: map[string][]types.PositionState{}},
		mockGovTallyStakingKeeper{},
		mockGovTallyAccountKeeper{},
	)

	_, _, err = tallyFn(s.ctx, s.app.GovKeeper, v1.Proposal{Id: testProposalID}, validators)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "invalid vote weight for validator")

	hasVote, hasErr := s.app.GovKeeper.Votes.Has(s.ctx, collections.Join(testProposalID, voterAddr))
	s.Require().NoError(hasErr)
	s.Require().True(hasVote, "failed validator tally should not remove the original vote")
}

// An undelegated tier position contributes zero tier voting power.
func (s *KeeperSuite) TestCustomTally_UndelegatedTierIgnored() {
	// Lock WITH delegation and immediate exit, then undelegate to get an undelegated position
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	s.advancePastExitDuration()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{Owner: delAddr.String(), PositionId: 0})
	s.Require().NoError(err)

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()
	totalPower, results := s.callCustomTally(testProposalID, validators)

	// No staking delegation, no delegated tier → zero
	s.Require().True(totalPower.IsZero(),
		"undelegated tier + no staking should be zero power; got %s", totalPower)
	s.Require().True(results[v1.OptionYes].IsZero())

	// Sanity: verify a delegated tier position gives non-zero power.
	// Create a new delegated position (the undelegated one can't be re-delegated
	// because exit has elapsed).
	pos2 := s.setupNewTierPosition(sdkmath.NewInt(5000), false)
	delAddr2 := sdk.MustAccAddressFromBech32(pos2.Owner)

	s.insertVote(testProposalID, delAddr2, yesVoteOpts())
	validators = s.buildValidatorsMap()
	tierPower := s.tierPowerFor(delAddr2, validators)
	s.Require().True(tierPower.IsPositive(), "delegated tier should give positive power")

	totalPower, _ = s.callCustomTally(testProposalID, validators)
	s.Require().True(totalPower.Equal(tierPower),
		"after delegating, tier power should appear; got %s, want %s", totalPower, tierPower)
}

// Two voters: one with tier, one without. Totals are correct and independent.
func (s *KeeperSuite) TestCustomTally_MultipleVoters() {
	pos := s.setupNewTierPositionWithDelegator(sdkmath.NewInt(6000), false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	pos2 := s.setupNewTierPosition(sdkmath.NewInt(3000), false)
	otherAddr := sdk.MustAccAddressFromBech32(pos2.Owner)

	// delAddr votes Yes, otherAddr votes No
	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	s.insertVote(testProposalID, otherAddr, noVoteOpts())

	validators := s.buildValidatorsMap()

	delStaking := s.stakingPowerFor(delAddr, valAddr, validators)
	delTier := s.tierPowerFor(delAddr, validators)
	otherTier := s.tierPowerFor(otherAddr, validators)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	expectedTotal := delStaking.Add(delTier).Add(otherTier)
	s.Require().True(totalPower.Equal(expectedTotal),
		"total should be delStaking + delTier + otherTier; got %s, want %s", totalPower, expectedTotal)

	expectedYes := delStaking.Add(delTier)
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes should include delAddr staking + tier; got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(otherTier),
		"No should equal otherAddr's tier; got %s, want %s", results[v1.OptionNo], otherTier)
}

// After tally, all processed votes are removed from the gov store.
func (s *KeeperSuite) TestCustomTally_VotesRemovedAfterTally() {
	pos1 := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	pos2 := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	addr1 := sdk.MustAccAddressFromBech32(pos1.Owner)
	addr2 := sdk.MustAccAddressFromBech32(pos2.Owner)

	s.insertVote(testProposalID, addr1, yesVoteOpts())
	s.insertVote(testProposalID, addr2, noVoteOpts())

	// Confirm votes exist before tally
	has1, _ := s.app.GovKeeper.Votes.Has(s.ctx, collections.Join(testProposalID, addr1))
	has2, _ := s.app.GovKeeper.Votes.Has(s.ctx, collections.Join(testProposalID, addr2))
	s.Require().True(has1)
	s.Require().True(has2)

	validators := s.buildValidatorsMap()
	s.callCustomTally(testProposalID, validators)

	// Votes should be gone
	has1, _ = s.app.GovKeeper.Votes.Has(s.ctx, collections.Join(testProposalID, addr1))
	has2, _ = s.app.GovKeeper.Votes.Has(s.ctx, collections.Join(testProposalID, addr2))
	s.Require().False(has1, "vote for addr1 should be removed after tally")
	s.Require().False(has2, "vote for addr2 should be removed after tally")
}

// Validator operator votes No alongside a tier voter voting Yes and a staking
// delegator voting Yes. The validator's second-pass tally (remaining
// delegator shares not yet counted) must be included correctly.
// After the double-count fix, the tier position's DelegatedShares are included
// in DelegatorDeductions, so the second-pass does not count them twice.
func (s *KeeperSuite) TestCustomTally_ValidatorVoteAlongsideTier() {
	delAddr, valAddr := s.getDelegator()

	pos := s.setupNewTierPosition(sdkmath.NewInt(4000), false)

	posAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	// Validator operator votes No → sets val.Vote in the first pass so the
	// second pass tallies the remaining (non-deducted) delegator shares.
	valAccAddr := sdk.AccAddress(valAddr)
	s.insertVote(testProposalID, valAccAddr, noVoteOpts())
	s.insertVote(testProposalID, posAddr, yesVoteOpts())
	s.insertVote(testProposalID, delAddr, yesVoteOpts())

	validators := s.buildValidatorsMap()
	valInfo := validators[valAddr.String()]

	// delAddr's staking power (first pass): their shares on the validator
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	delStakingPower := del.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	// valAccAddr's first-pass contribution
	valSelfShares := sdkmath.LegacyZeroDec()
	valSelfPower := sdkmath.LegacyZeroDec()
	if selfDel, selfErr := s.app.StakingKeeper.GetDelegation(s.ctx, valAccAddr, valAddr); selfErr == nil {
		valSelfShares = selfDel.Shares
		valSelfPower = selfDel.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)
	}

	// Get tier position's DelegatedShares (deducted from validator in the fix).
	tierPositions, err := s.keeper.GetPositionStatesByOwner(s.ctx, posAddr)
	s.Require().NoError(err)
	s.Require().Len(tierPositions, 1, "posAddr should have one active tier position")
	tierPos := tierPositions[0]
	positionAmount := tierPos.Delegation.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	// Second pass: validator tallies remaining shares. After the fix, deductions
	// include: delAddr.Shares + valSelfShares + tierPos.Delegation.Shares.
	totalDeductions := del.Shares.Add(valSelfShares).Add(tierPos.Delegation.Shares)
	valRemainingPower := valInfo.DelegatorShares.Sub(totalDeductions).
		MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	expectedYes := delStakingPower.Add(positionAmount)
	expectedNo := valSelfPower.Add(valRemainingPower)
	expectedTotal := expectedYes.Add(expectedNo)

	s.Require().True(totalPower.Equal(expectedTotal),
		"total power mismatch; got %s, want %s", totalPower, expectedTotal)
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes mismatch; got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(expectedNo),
		"No mismatch; got %s, want %s", results[v1.OptionNo], expectedNo)
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

// No votes at all → zero totals, empty results.
func (s *KeeperSuite) TestCustomTally_NoVotes() {
	s.setupTier(1)
	validators := s.buildValidatorsMap()
	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.IsZero())
	s.Require().True(results[v1.OptionYes].IsZero())
	s.Require().True(results[v1.OptionNo].IsZero())
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

// TestCustomTally_DoubleCountPrevented verifies that when a tier voter and the
// validator both vote, the tier position's delegation shares are deducted from
// the validator's second-pass calculation, preventing double-counting of the
// same economic stake.
func (s *KeeperSuite) TestCustomTally_DoubleCountPrevented() {
	tierAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(tierAmount, false)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	valAccAddr := sdk.AccAddress(valAddr)
	// Only the tier voter and the validator itself vote.
	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	s.insertVote(testProposalID, valAccAddr, noVoteOpts())

	validators := s.buildValidatorsMap()
	valInfo := validators[valAddr.String()]

	// Get the tier position's DelegatedShares.
	tierPositions, err := s.keeper.GetPositionStatesByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(tierPositions, 1)
	tierPos := tierPositions[0]
	positionAmount := tierPos.Delegation.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	// Validator self-delegation (if any).
	valSelfShares := sdkmath.LegacyZeroDec()
	valSelfPower := sdkmath.LegacyZeroDec()
	if selfDel, selfErr := s.app.StakingKeeper.GetDelegation(s.ctx, valAccAddr, valAddr); selfErr == nil {
		valSelfShares = selfDel.Shares
		valSelfPower = selfDel.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)
	}

	// With the fix, second pass deductions = valSelfShares + tierPos.Delegation.Shares.
	// The module account's shares are not counted a second time.
	totalDeductions := valSelfShares.Add(tierPos.Delegation.Shares)
	valRemainingPower := valInfo.DelegatorShares.Sub(totalDeductions).
		MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	expectedYes := positionAmount
	expectedNo := valSelfPower.Add(valRemainingPower)
	expectedTotal := expectedYes.Add(expectedNo)

	s.Require().True(totalPower.Equal(expectedTotal),
		"double-count check: total mismatch; got %s, want %s", totalPower, expectedTotal)
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes should equal only tier power (no double-count); got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(expectedNo),
		"No should equal validator second-pass (self + remaining); got %s, want %s", results[v1.OptionNo], expectedNo)

	// Sanity: in a 1:1 token-to-share ratio test environment, tier token value
	// should equal the locked amount.
	s.Require().True(positionAmount.Equal(sdkmath.LegacyNewDecFromInt(tierAmount)),
		"in 1:1 ratio test env, tier power should equal position amount; got %s", positionAmount)
}

// TestCustomTally_ExitingTierPositionIncluded verifies that a tier position
// with a triggered exit still contributes voting power.
func (s *KeeperSuite) TestCustomTally_ExitingTierPositionIncluded() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	// Verify position state: delegated but exiting.
	allPositions, err := s.keeper.GetPositionStatesByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(allPositions, 1)
	s.Require().True(allPositions[0].IsDelegated(), "position should still be delegated")
	s.Require().True(allPositions[0].HasTriggeredExit(), "position should have triggered exit")

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()

	// Per ADR-006 §8.5, exiting-but-delegated positions still count for
	// governance voting power.
	expectedPower := s.tierPowerFor(delAddr, validators)
	s.Require().True(expectedPower.IsPositive(),
		"exiting but delegated position should have positive power")

	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(expectedPower),
		"exiting tier should contribute voting power; got %s, want %s", totalPower, expectedPower)
	s.Require().True(results[v1.OptionYes].Equal(expectedPower),
		"Yes should include exiting tier position; got %s", results[v1.OptionYes])
}

// TestCustomTally_TierKeeperError verifies that an error from the tier keeper
// is propagated correctly by the tally function.
func (s *KeeperSuite) TestCustomTally_TierKeeperError() {
	_, bondDenom := s.getStakingData()

	// Set up a voter with a vote in the store.
	addr := sdk.AccAddress([]byte("error_tier_voter____"))
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))))
	s.Require().NoError(err)
	s.insertVote(testProposalID, addr, yesVoteOpts())

	// Build the tally function with a mock that always errors.
	sentinel := errors.New("tier keeper error")
	errMock := mockTierVotingPower{
		positions: map[string][]types.PositionState{},
		getErr:    sentinel,
	}
	tallyFn := keeper.NewCustomTallyTierVotesFn(
		errMock, s.app.StakingKeeper, s.app.AccountKeeper,
	)
	proposal := v1.Proposal{Id: testProposalID}
	validators := s.buildValidatorsMap()

	_, _, tallyErr := tallyFn(s.ctx, s.app.GovKeeper, proposal, validators)
	s.Require().Error(tallyErr, "tally should propagate tier keeper error")
	s.Require().ErrorContains(tallyErr, "error getting tier positions for")
}

// TestCustomTally_MultiplePositionsSameValidator verifies that a voter with
// two tier positions on the same validator has both positions' DelegatedShares
// counted once each — neither double-counted nor dropped.
func (s *KeeperSuite) TestCustomTally_MultiplePositionsSameValidator() {
	// Position 1: 3000 tokens on the same validator.
	lockAmt1 := sdkmath.NewInt(3000)
	pos1 := s.setupNewTierPosition(lockAmt1, false)
	delAddr := sdk.MustAccAddressFromBech32(pos1.Owner)
	valAddr1 := sdk.MustValAddressFromBech32(pos1.Delegation.ValidatorAddress)

	// Position 2: 5000 tokens on the same validator.
	lockAmt2 := sdkmath.NewInt(5000)
	_, bondDenom := s.getStakingData()
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmt2)))
	s.Require().NoError(err)
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           lockAmt2,
		ValidatorAddress: valAddr1.String(),
	})
	s.Require().NoError(err)

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()

	// tierPowerFor sums both positions' DelegatedShares-to-tokens.
	expected := s.tierPowerFor(delAddr, validators)
	s.Require().True(expected.IsPositive(), "two delegated positions should give positive power")

	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(expected),
		"total power should equal sum of both positions; got %s, want %s", totalPower, expected)
	s.Require().True(results[v1.OptionYes].Equal(expected),
		"Yes should equal combined tier power; got %s, want %s", results[v1.OptionYes], expected)
	s.Require().True(results[v1.OptionNo].IsZero())
}

// TestCustomTally_TierPositionOnUnbondingValidatorNotCounted verifies that a
// tier position whose validator is unbonding/unbonded contributes zero voting
// power, consistent with standard gov tally semantics.
func (s *KeeperSuite) TestCustomTally_TierPositionOnUnbondingValidatorNotCounted() {
	tierAmount := sdkmath.NewInt(5000)
	pos := s.setupNewTierPosition(tierAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)

	s.jailAndUnbondValidator(valAddr)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "validator should no longer be bonded")

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()
	s.Require().NotContains(validators, valAddr.String(),
		"unbonding validator should not be in bonded validators map")

	tierPositions, err := s.keeper.GetPositionStatesByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(tierPositions, 1)

	totalPower, results := s.callCustomTally(testProposalID, validators)
	s.Require().True(totalPower.IsZero(),
		"tier position on unbonding validator should not count; got %s", totalPower)
	s.Require().True(results[v1.OptionYes].IsZero(),
		"Yes should be zero for unbonding validator tier position; got %s", results[v1.OptionYes])
}

// TestCustomTally_ExitingPositionDoubleCountPrevented verifies that exiting
// tier positions (which now contribute voting power per ADR-006 §8.5) still
// have their DelegatedShares correctly deducted from the validator's
// second-pass tally to prevent double-counting.
func (s *KeeperSuite) TestCustomTally_ExitingPositionDoubleCountPrevented() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(5000), true)
	delAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	valAccAddr := sdk.AccAddress(valAddr)

	// Tier voter votes Yes, validator votes No.
	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	s.insertVote(testProposalID, valAccAddr, noVoteOpts())

	validators := s.buildValidatorsMap()
	valInfo := validators[valAddr.String()]

	// The exiting position is still delegated, so it should contribute.
	tierPositions, err := s.keeper.GetPositionStatesByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(tierPositions, 1, "exiting position should be active for governance")
	tierPos := tierPositions[0]
	positionAmount := tierPos.Delegation.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	// Validator self-delegation.
	valSelfShares := sdkmath.LegacyZeroDec()
	valSelfPower := sdkmath.LegacyZeroDec()
	if selfDel, selfErr := s.app.StakingKeeper.GetDelegation(s.ctx, valAccAddr, valAddr); selfErr == nil {
		valSelfShares = selfDel.Shares
		valSelfPower = selfDel.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)
	}

	// With the fix, second pass deductions = valSelfShares + tierPos.Delegation.Shares.
	totalDeductions := valSelfShares.Add(tierPos.Delegation.Shares)
	valRemainingPower := valInfo.DelegatorShares.Sub(totalDeductions).
		MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	expectedYes := positionAmount
	expectedNo := valSelfPower.Add(valRemainingPower)
	expectedTotal := expectedYes.Add(expectedNo)

	s.Require().True(totalPower.Equal(expectedTotal),
		"exiting position double-count check: total mismatch; got %s, want %s", totalPower, expectedTotal)
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes should equal only exiting tier power; got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(expectedNo),
		"No should equal validator second-pass; got %s, want %s", results[v1.OptionNo], expectedNo)
}

// TestCustomTally_TierPositionValidatorNotInMap verifies that a tier position
// whose validator is not in the bonded validators map is gracefully skipped,
// contributing zero voting power without panicking or returning an error.
func (s *KeeperSuite) TestCustomTally_TierPositionValidatorNotInMap() {
	// Set up one real delegated position via the full LockTier path so we
	// have a baseline non-zero contribution.
	realPos := s.setupNewTierPosition(sdkmath.NewInt(1000), false)
	voter := sdk.MustAccAddressFromBech32(realPos.Owner)

	// Synthetic extra position for the same voter, delegated to a validator
	// not registered in staking — positionVotingPower must short-circuit.
	fakeValAddr := sdk.ValAddress([]byte("not_in_bonded_map___"))
	now := s.ctx.BlockTime()
	bogusPos := types.PositionState{
		Position: types.NewPosition(999, realPos.Owner, 1, 1, 0, now, true, now),
		Delegation: &stakingtypes.Delegation{
			DelegatorAddress: types.GetDelegatorAddress(999).String(),
			ValidatorAddress: fakeValAddr.String(),
			Shares:           sdkmath.LegacyNewDec(5000),
		},
	}

	// Mock returns BOTH positions so the tally has to iterate and skip the
	// bogus one while accepting the real one. Using the keeper directly
	// wouldn't let us inject a delegation to a non-existent validator.
	mock := mockTierVotingPower{
		positions: map[string][]types.PositionState{
			voter.String(): {realPos, bogusPos},
		},
	}

	s.insertVote(testProposalID, voter, yesVoteOpts())
	validators := s.buildValidatorsMap()

	// Expected contribution from the real position only (same formula the
	// tally uses). tierPowerFor walks the real keeper which only knows about
	// realPos — bogusPos isn't persisted.
	expected := s.tierPowerFor(voter, validators)
	s.Require().True(expected.IsPositive(),
		"baseline real-validator tier power must be positive; got %s", expected)

	tallyFn := keeper.NewCustomTallyTierVotesFn(mock, s.app.StakingKeeper, s.app.AccountKeeper)
	proposal := v1.Proposal{Id: testProposalID}
	totalPower, results, err := tallyFn(s.ctx, s.app.GovKeeper, proposal, validators)
	s.Require().NoError(err)

	// Total must equal the real position's contribution;
	// the voting power should only include the real position.
	s.Require().True(totalPower.Equal(expected),
		"total power should equal real position contribution; got %s, want %s", totalPower, expected)
	s.Require().True(results[v1.OptionYes].Equal(expected),
		"Yes should equal real position contribution; got %s, want %s", results[v1.OptionYes], expected)
}
