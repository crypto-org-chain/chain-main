package keeper_test

import (
	"context"
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock + standalone tests
// ---------------------------------------------------------------------------

type mockTierVotingPower struct {
	powers map[string]sdkmath.LegacyDec
}

func (m mockTierVotingPower) GetVotingPowerForAddress(_ context.Context, voter sdk.AccAddress) (sdkmath.LegacyDec, error) {
	if p, ok := m.powers[voter.String()]; ok {
		return p, nil
	}
	return sdkmath.LegacyZeroDec(), nil
}

var _ keeper.TierVotingPowerProvider = mockTierVotingPower{}

func TestNewCalculateVoteResultsAndVotingPowerFn_NotNil(t *testing.T) {
	mock := mockTierVotingPower{powers: map[string]sdkmath.LegacyDec{}}
	fn := keeper.NewCalculateVoteResultsAndVotingPowerFn(mock, nil, nil)
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
	tallyFn := keeper.NewCalculateVoteResultsAndVotingPowerFn(
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

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

// A voter with NO staking delegation but WITH a delegated tier position.
// All voting power comes from tier.
func (s *KeeperSuite) TestCustomTally_TierOnlyVoter() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	freshAddr := sdk.AccAddress([]byte("fresh_tier_voter____"))
	tierAmount := sdkmath.NewInt(8000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, freshAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, tierAmount)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           tierAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.insertVote(testProposalID, freshAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()
	totalPower, results := s.callCustomTally(testProposalID, validators)

	expected := sdkmath.LegacyNewDecFromInt(tierAmount)
	s.Require().True(totalPower.Equal(expected),
		"voter with no staking should have power = tier amount; got %s, want %s", totalPower, expected)
	s.Require().True(results[v1.OptionYes].Equal(expected))
	s.Require().True(results[v1.OptionNo].IsZero())
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

// A voter with BOTH staking delegation and a delegated tier position.
// Total = staking power + tier power.
func (s *KeeperSuite) TestCustomTally_StakingPlusTier() {
	delAddr, valAddr, _ := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	tierAmount := sdkmath.NewInt(5000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           tierAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()

	expectedStaking := s.stakingPowerFor(delAddr, valAddr, validators)
	s.Require().True(expectedStaking.IsPositive(), "delegator should have staking power")
	expectedTier := sdkmath.LegacyNewDecFromInt(tierAmount)
	expectedTotal := expectedStaking.Add(expectedTier)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(expectedTotal),
		"total should be staking + tier; got %s, want %s", totalPower, expectedTotal)
	s.Require().True(results[v1.OptionYes].Equal(expectedTotal))
}

// Weighted vote (60 % Yes / 40 % No) splits BOTH staking and tier power.
func (s *KeeperSuite) TestCustomTally_WeightedVoteSplitsTierPower() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	freshAddr := sdk.AccAddress([]byte("weighted_voter______"))
	tierAmount := sdkmath.NewInt(10000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, freshAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, tierAmount)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           tierAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	opts := []*v1.WeightedVoteOption{
		{Option: v1.OptionYes, Weight: "0.600000000000000000"},
		{Option: v1.OptionNo, Weight: "0.400000000000000000"},
	}
	s.insertVote(testProposalID, freshAddr, opts)
	validators := s.buildValidatorsMap()
	totalPower, results := s.callCustomTally(testProposalID, validators)

	tierDec := sdkmath.LegacyNewDecFromInt(tierAmount)
	s.Require().True(totalPower.Equal(tierDec),
		"total should equal tier amount; got %s, want %s", totalPower, tierDec)

	expectedYes := tierDec.Mul(sdkmath.LegacyNewDecWithPrec(6, 1))  // 60 %
	expectedNo := tierDec.Mul(sdkmath.LegacyNewDecWithPrec(4, 1))   // 40 %
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes should be 60%% of tier; got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(expectedNo),
		"No should be 40%% of tier; got %s, want %s", results[v1.OptionNo], expectedNo)
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}

// An undelegated tier position contributes zero tier voting power.
func (s *KeeperSuite) TestCustomTally_UndelegatedTierIgnored() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	freshAddr := sdk.AccAddress([]byte("undel_tier_voter____"))
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, freshAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(5000))))
	s.Require().NoError(err)

	// Lock WITHOUT delegation
	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  freshAddr.String(),
		Id:     1,
		Amount: sdkmath.NewInt(5000),
	})
	s.Require().NoError(err)

	s.insertVote(testProposalID, freshAddr, yesVoteOpts())
	validators := s.buildValidatorsMap()
	totalPower, results := s.callCustomTally(testProposalID, validators)

	// No staking delegation, no delegated tier → zero
	s.Require().True(totalPower.IsZero(),
		"undelegated tier + no staking should be zero power; got %s", totalPower)
	s.Require().True(results[v1.OptionYes].IsZero())

	// Sanity: verify the same address with delegated tier would be non-zero.
	// (Re-insert vote since tally removed it, then delegate the position.)
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      freshAddr.String(),
		PositionId: 0,
		Validator:  valAddr.String(),
	})
	s.Require().NoError(err)

	s.insertVote(testProposalID, freshAddr, yesVoteOpts())
	validators = s.buildValidatorsMap()
	totalPower, _ = s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.Equal(sdkmath.LegacyNewDec(5000)),
		"after delegating, tier power should appear; got %s", totalPower)
}

// Two voters: one with tier, one without. Totals are correct and independent.
func (s *KeeperSuite) TestCustomTally_MultipleVoters() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	tierAmount := sdkmath.NewInt(6000)
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            delAddr.String(),
		Id:               1,
		Amount:           tierAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Second voter: only tier, no staking
	otherAddr := sdk.AccAddress([]byte("other_tier_voter____"))
	otherTier := sdkmath.NewInt(3000)
	err = banktestutil.FundAccount(s.ctx, s.app.BankKeeper, otherAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, otherTier)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            otherAddr.String(),
		Id:               1,
		Amount:           otherTier,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// delAddr votes Yes, otherAddr votes No
	s.insertVote(testProposalID, delAddr, yesVoteOpts())
	s.insertVote(testProposalID, otherAddr, noVoteOpts())

	validators := s.buildValidatorsMap()

	delStaking := s.stakingPowerFor(delAddr, valAddr, validators)
	delTier := sdkmath.LegacyNewDecFromInt(tierAmount)
	otherTierDec := sdkmath.LegacyNewDecFromInt(otherTier)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	expectedTotal := delStaking.Add(delTier).Add(otherTierDec)
	s.Require().True(totalPower.Equal(expectedTotal),
		"total should be delStaking + delTier + otherTier; got %s, want %s", totalPower, expectedTotal)

	expectedYes := delStaking.Add(delTier)
	s.Require().True(results[v1.OptionYes].Equal(expectedYes),
		"Yes should include delAddr staking + tier; got %s, want %s", results[v1.OptionYes], expectedYes)
	s.Require().True(results[v1.OptionNo].Equal(otherTierDec),
		"No should equal otherAddr's tier; got %s, want %s", results[v1.OptionNo], otherTierDec)
}

// After tally, all processed votes are removed from the gov store.
func (s *KeeperSuite) TestCustomTally_VotesRemovedAfterTally() {
	_, _, bondDenom := s.setupTierAndDelegator()

	addr1 := sdk.AccAddress([]byte("remove_voter_1______"))
	addr2 := sdk.AccAddress([]byte("remove_voter_2______"))
	for _, addr := range []sdk.AccAddress{addr1, addr2} {
		err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr,
			sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1000))))
		s.Require().NoError(err)
	}

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
func (s *KeeperSuite) TestCustomTally_ValidatorVoteAlongsideTier() {
	delAddr, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	freshAddr := sdk.AccAddress([]byte("tier_voter_valtest__"))
	tierAmount := sdkmath.NewInt(4000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, freshAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, tierAmount)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            freshAddr.String(),
		Id:               1,
		Amount:           tierAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	// Validator operator votes No → sets val.Vote in the first pass so the
	// second pass tallies the remaining (non-deducted) delegator shares.
	valAccAddr := sdk.AccAddress(valAddr)
	s.insertVote(testProposalID, valAccAddr, noVoteOpts())
	s.insertVote(testProposalID, freshAddr, yesVoteOpts())
	s.insertVote(testProposalID, delAddr, yesVoteOpts())

	validators := s.buildValidatorsMap()
	valInfo := validators[valAddr.String()]

	// delAddr's staking power (first pass): their shares on the validator
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	delStakingPower := del.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	// valAccAddr's first-pass contribution: it may or may not have a
	// personal delegation — gather whatever deduction it causes.
	valSelfShares := sdkmath.LegacyZeroDec()
	valSelfPower := sdkmath.LegacyZeroDec()
	if selfDel, selfErr := s.app.StakingKeeper.GetDelegation(s.ctx, valAccAddr, valAddr); selfErr == nil {
		valSelfShares = selfDel.Shares
		valSelfPower = selfDel.Shares.MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)
	}

	// Second pass: validator tallies remaining shares (delegators that did
	// not vote, including the tier module delegation).
	totalDeductions := del.Shares.Add(valSelfShares)
	valRemainingPower := valInfo.DelegatorShares.Sub(totalDeductions).
		MulInt(valInfo.BondedTokens).Quo(valInfo.DelegatorShares)

	tierDec := sdkmath.LegacyNewDecFromInt(tierAmount)

	totalPower, results := s.callCustomTally(testProposalID, validators)

	expectedYes := delStakingPower.Add(tierDec)
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
	s.setupTierAndDelegator()
	validators := s.buildValidatorsMap()
	totalPower, results := s.callCustomTally(testProposalID, validators)

	s.Require().True(totalPower.IsZero())
	s.Require().True(results[v1.OptionYes].IsZero())
	s.Require().True(results[v1.OptionNo].IsZero())
	s.Require().True(results[v1.OptionAbstain].IsZero())
	s.Require().True(results[v1.OptionNoWithVeto].IsZero())
}
