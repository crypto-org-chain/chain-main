package keeper

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	addresscodec "cosmossdk.io/core/address"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TierVotingPowerProvider is the interface the custom gov tally needs from
// the tiered rewards module.
type TierVotingPowerProvider interface {
	GetPositionStatesByOwner(ctx context.Context, owner sdk.AccAddress) ([]types.PositionState, error)
}

// GovTallyStakingKeeper is the subset of staking keeper needed by the custom tally function.
type GovTallyStakingKeeper interface {
	ValidatorAddressCodec() addresscodec.Codec
	IterateDelegations(ctx context.Context, delegator sdk.AccAddress, fn func(index int64, delegation stakingtypes.DelegationI) (stop bool)) error
}

// GovTallyAccountKeeper is the subset of account keeper needed by the custom tally function.
type GovTallyAccountKeeper interface {
	AddressCodec() addresscodec.Codec
}

// NewCustomTallyTierVotesFn returns a tally function that includes
// tier-delegated voting power for each voter in addition to standard staking power.
// To prevent double-counting, each position's DelegatedShares are added to the
// validator's DelegatorDeductions before the second-pass tally.
func NewCustomTallyTierVotesFn(
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

			// Tier-delegated voting power:
			// - compute from position delegation/share state via keeper-level helper
			// - still deduct DelegatedShares from bonded validator second-pass tally
			//   when the validator is present in the gov validator map
			positions, err := tierKeeper.GetPositionStatesByOwner(ctx, voter)
			if err != nil {
				return false, fmt.Errorf("error getting tier positions for %s: %w", vote.Voter, err)
			}
			for _, pos := range positions {
				posPower := positionVotingPower(pos, validators)
				if posPower.IsZero() {
					continue
				}

				valAddr := pos.Delegation.ValidatorAddress
				if val, ok := validators[valAddr]; ok {
					val.DelegatorDeductions = val.DelegatorDeductions.Add(pos.Delegation.Shares)
					validators[valAddr] = val
				}

				if err := distributeVotingPower(vote.Options, posPower, results); err != nil {
					return false, fmt.Errorf("invalid vote weight for voter %s: %w", vote.Voter, err)
				}
				totalVotingPower = totalVotingPower.Add(posPower)
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
