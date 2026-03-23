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

// NewCalculateVoteResultsAndVotingPowerFn returns a CalculateVoteResultsAndVotingPowerFn
// that includes tier-delegated voting power for each voter in addition to
// standard staking power. The returned function replicates the SDK default
// tally logic and, for every voter, adds their tier voting power (sum of
// DelegatedShares converted to tokens for active tier positions owned by the voter).
//
// To prevent double-counting, the tier position's DelegatedShares are added to
// the validator's DelegatorDeductions before the second-pass tally. This ensures
// the module account's delegation on the validator is not counted a second time
// through the validator's residual delegator shares.
//
// Because the gov Keeper's staking/auth keepers are unexported, the callers
// (app.go) must pass them explicitly so the custom function can iterate
// delegations and decode addresses.
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
		// Collect keys to remove after iteration; deleting during Walk is not safe.
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
			err = sk.IterateDelegations(ctx, voter, func(_ int64, delegation stakingtypes.DelegationI) (stop bool) {
				valAddrStr := delegation.GetValidatorAddr()

				if val, ok := validators[valAddrStr]; ok {
					val.DelegatorDeductions = val.DelegatorDeductions.Add(delegation.GetShares())
					validators[valAddrStr] = val

					votingPower := delegation.GetShares().MulInt(val.BondedTokens).Quo(val.DelegatorShares)
					distributeVotingPower(vote.Options, votingPower, results)
					totalVotingPower = totalVotingPower.Add(votingPower)
				}

				return false
			})
			if err != nil {
				return false, err
			}

			// Tier-delegated voting power: for each active tier position, deduct
			// its DelegatedShares from the validator so the module account's
			// delegation is not double-counted in the validator second-pass tally.
			// Power is computed using the same shares-to-tokens formula as staking.
			tierPositions, err := tierKeeper.GetActiveDelegatedPositionsByOwner(ctx, voter)
			if err != nil {
				return false, fmt.Errorf("error getting tier positions for %s: %w", vote.Voter, err)
			}
			for _, pos := range tierPositions {
				val, ok := validators[pos.Validator]
				if !ok || val.DelegatorShares.IsZero() {
					continue
				}
				// Deduct so the second pass does not count this delegation again.
				val.DelegatorDeductions = val.DelegatorDeductions.Add(pos.DelegatedShares)
				validators[pos.Validator] = val

				tierPower := pos.DelegatedShares.MulInt(val.BondedTokens).Quo(val.DelegatorShares)
				distributeVotingPower(vote.Options, tierPower, results)
				totalVotingPower = totalVotingPower.Add(tierPower)
			}

			votesToRemove = append(votesToRemove, key)
			return false, nil
		})
		if err != nil {
			return math.LegacyZeroDec(), nil, fmt.Errorf("error while iterating votes: %w", err)
		}

		for _, key := range votesToRemove {
			if err := k.Votes.Remove(ctx, key); err != nil {
				return math.LegacyDec{}, nil, fmt.Errorf("error while removing vote (%d/%s): %w", key.K1(), key.K2(), err)
			}
		}

		// Tally remaining validator voting power (second pass): for validators who
		// voted themselves, attribute the shares not already deducted by individual
		// delegators to the validator's own vote choice.
		for _, val := range validators {
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

			distributeVotingPower(val.Vote, votingPower, results)
			totalVotingPower = totalVotingPower.Add(votingPower)
		}

		return totalVotingPower, results, nil
	}
}

// distributeVotingPower splits power across weighted vote options and accumulates
// into results. Weight strings are validated at vote-submission time by the gov
// module, so parse errors here indicate corrupted state and are treated as zero
// weight (consistent with the SDK default tally behaviour).
func distributeVotingPower(options []*v1.WeightedVoteOption, power math.LegacyDec, results map[v1.VoteOption]math.LegacyDec) {
	for _, option := range options {
		weight, _ := math.LegacyNewDecFromStr(option.Weight)
		results[option.Option] = results[option.Option].Add(power.Mul(weight))
	}
}
