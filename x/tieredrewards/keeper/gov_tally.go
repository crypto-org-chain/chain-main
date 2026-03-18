package keeper

import (
	"context"
	"fmt"

	addresscodec "cosmossdk.io/core/address"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TierVotingPowerProvider is the interface the custom gov tally needs from the
// tiered rewards module.
type TierVotingPowerProvider interface {
	GetVotingPowerForAddress(ctx context.Context, voter sdk.AccAddress) (math.LegacyDec, error)
}

// GovTallyStakingKeeper is the subset of staking keeper needed by the custom
// tally function (matches gov's types.StakingKeeper).
type GovTallyStakingKeeper interface {
	ValidatorAddressCodec() addresscodec.Codec
	IterateDelegations(ctx context.Context, delegator sdk.AccAddress, fn func(index int64, delegation stakingtypes.DelegationI) (stop bool)) error
}

// GovTallyAccountKeeper is the subset of account keeper needed by the custom
// tally function.
type GovTallyAccountKeeper interface {
	AddressCodec() addresscodec.Codec
}

// NewCalculateVoteResultsAndVotingPowerFn returns a CalculateVoteResultsAndVotingPowerFn
// that includes tier-delegated voting power for each voter in addition to
// standard staking power. The returned function replicates the SDK default
// tally logic and, for every voter, adds the voter's tier voting power (sum of
// AmountLocked for delegated positions owned by the voter).
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

					for _, option := range vote.Options {
						weight, _ := math.LegacyNewDecFromStr(option.Weight)
						subPower := votingPower.Mul(weight)
						results[option.Option] = results[option.Option].Add(subPower)
					}
					totalVotingPower = totalVotingPower.Add(votingPower)
				}

				return false
			})
			if err != nil {
				return false, err
			}

			// Tier-delegated voting power: add power from delegated tier positions
			tierPower, tierErr := tierKeeper.GetVotingPowerForAddress(ctx, voter)
			if tierErr != nil {
				return false, fmt.Errorf("error getting tier voting power for %s: %w", vote.Voter, tierErr)
			}
			if tierPower.IsPositive() {
				for _, option := range vote.Options {
					weight, _ := math.LegacyNewDecFromStr(option.Weight)
					subPower := tierPower.Mul(weight)
					results[option.Option] = results[option.Option].Add(subPower)
				}
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

		// Tally remaining validator voting power
		for _, val := range validators {
			if len(val.Vote) == 0 {
				continue
			}

			sharesAfterDeductions := val.DelegatorShares.Sub(val.DelegatorDeductions)
			votingPower := sharesAfterDeductions.MulInt(val.BondedTokens).Quo(val.DelegatorShares)

			for _, option := range val.Vote {
				weight, _ := math.LegacyNewDecFromStr(option.Weight)
				subPower := votingPower.Mul(weight)
				results[option.Option] = results[option.Option].Add(subPower)
			}
			totalVotingPower = totalVotingPower.Add(votingPower)
		}

		return totalVotingPower, results, nil
	}
}
