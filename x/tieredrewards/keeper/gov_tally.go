package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// NewCalculateVoteResultsAndVotingPowerFn returns a tally function that includes
// tier-delegated voting power for each voter in addition to standard staking power.
// To prevent double-counting, each position's DelegatedShares are added to the
// validator's DelegatorDeductions before the second-pass tally.
func NewCalculateVoteResultsAndVotingPowerFn(
	tierKeeper TierVotingPowerProvider,
	sk GovTallyStakingKeeper,
	ak GovTallyAccountKeeper,
) govkeeper.CalculateVoteResultsAndVotingPowerFn {
	return func(
		ctx context.Context,
		k govkeeper.Keeper,
		proposal v1.Proposal,
		validators map[string]v1.ValidatorGovInfo,
	) (totalVoterPower math.LegacyDec, results map[v1.VoteOption]math.LegacyDec, err error) {
		totalVotingPower := math.LegacyZeroDec()

		results = make(map[v1.VoteOption]math.LegacyDec)
		results[v1.OptionYes] = math.LegacyZeroDec()
		results[v1.OptionAbstain] = math.LegacyZeroDec()
		results[v1.OptionNo] = math.LegacyZeroDec()
		results[v1.OptionNoWithVeto] = math.LegacyZeroDec()

		rng := collections.NewPrefixedPairRange[uint64, sdk.AccAddress](proposal.Id)
		votesToRemove := []collections.Pair[uint64, sdk.AccAddress]{}

		err = k.Votes.Walk(ctx, rng, func(key collections.Pair[uint64, sdk.AccAddress], vote v1.Vote) (bool, error) {
			voter, err := ak.AddressCodec().StringToBytes(vote.Voter)
			if err != nil {
				return false, err
			}

			valAddrStr, err := sk.ValidatorAddressCodec().BytesToString(voter)
			if err != nil {
				return false, err
			}
			if val, ok := validators[valAddrStr]; ok {
				val.Vote = vote.Options
				validators[valAddrStr] = val
			}

			// Standard staking voting power
			var voteWeightErr error
			err = sk.IterateDelegations(ctx, voter, func(_ int64, delegation stakingtypes.DelegationI) (stop bool) {
				valAddrStr := delegation.GetValidatorAddr()

				if val, ok := validators[valAddrStr]; ok {
					val.DelegatorDeductions = val.DelegatorDeductions.Add(delegation.GetShares())
					validators[valAddrStr] = val

					votingPower := delegation.GetShares().MulInt(val.BondedTokens).Quo(val.DelegatorShares)
					if err := distributeVotingPower(vote.Options, votingPower, results); err != nil {
						voteWeightErr = fmt.Errorf("invalid vote weight for voter %s: %w", vote.Voter, err)
						return true
					}
					totalVotingPower = totalVotingPower.Add(votingPower)
				}

				return false
			})
			if voteWeightErr != nil {
				return false, voteWeightErr
			}
			if err != nil {
				return false, err
			}

			// Tier-delegated voting power: deduct DelegatedShares from each validator
			// so the module account's delegation is not double-counted.
			tierPositions, err := tierKeeper.GetDelegatedPositionsByOwner(ctx, voter)
			if err != nil {
				return false, fmt.Errorf("error getting tier positions for %s: %w", vote.Voter, err)
			}
			for _, pos := range tierPositions {
				val, ok := validators[pos.Validator]
				if !ok || val.DelegatorShares.IsZero() {
					continue
				}
				val.DelegatorDeductions = val.DelegatorDeductions.Add(pos.DelegatedShares)
				validators[pos.Validator] = val

				tierPower := pos.DelegatedShares.MulInt(val.BondedTokens).Quo(val.DelegatorShares)
				if err := distributeVotingPower(vote.Options, tierPower, results); err != nil {
					return false, fmt.Errorf("invalid vote weight for voter %s: %w", vote.Voter, err)
				}
				totalVotingPower = totalVotingPower.Add(tierPower)
			}

			votesToRemove = append(votesToRemove, key)
			return false, nil
		})
		if err != nil {
			return math.LegacyZeroDec(), nil, fmt.Errorf("error while iterating votes: %w", err)
		}

		// Second pass: attribute remaining validator shares to the validator's own vote.
		for valAddrStr, val := range validators {
			if len(val.Vote) == 0 {
				continue
			}
			if val.DelegatorShares.IsZero() {
				continue
			}

			sharesAfterDeductions := val.DelegatorShares.Sub(val.DelegatorDeductions)
			if sharesAfterDeductions.IsNegative() {
				sharesAfterDeductions = math.LegacyZeroDec()
			}
			votingPower := sharesAfterDeductions.MulInt(val.BondedTokens).Quo(val.DelegatorShares)

			if err := distributeVotingPower(val.Vote, votingPower, results); err != nil {
				return math.LegacyZeroDec(), nil, fmt.Errorf("invalid vote weight for validator %s: %w", valAddrStr, err)
			}
			totalVotingPower = totalVotingPower.Add(votingPower)
		}

		for _, key := range votesToRemove {
			if err := k.Votes.Remove(ctx, key); err != nil {
				return math.LegacyDec{}, nil, fmt.Errorf("error while removing vote (%d/%s): %w", key.K1(), key.K2(), err)
			}
		}

		return totalVotingPower, results, nil
	}
}

func distributeVotingPower(options []*v1.WeightedVoteOption, power math.LegacyDec, results map[v1.VoteOption]math.LegacyDec) error {
	for _, option := range options {
		weight, err := math.LegacyNewDecFromStr(option.Weight)
		if err != nil {
			return fmt.Errorf("option %s has invalid weight %q: %w", option.Option.String(), option.Weight, err)
		}
		results[option.Option] = results[option.Option].Add(power.Mul(weight))
	}
	return nil
}
