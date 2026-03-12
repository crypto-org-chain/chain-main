package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	addresscodec "cosmossdk.io/core/address"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GovStakingKeeper defines the staking keeper methods needed for governance tally.
// This matches the gov module's expected StakingKeeper interface.
type GovStakingKeeper interface {
	ValidatorAddressCodec() addresscodec.Codec
	IterateDelegations(ctx context.Context, delegator sdk.AccAddress,
		fn func(index int64, delegation stakingtypes.DelegationI) (stop bool)) error
}

// CustomTallyFn returns a custom CalculateVoteResultsAndVotingPowerFn that includes
// tier lock voting power in governance tallies.
// It copies the default tally logic and adds tier voting power for each voter.
// The ac parameter is the account address codec used to decode voter addresses.
func CustomTallyFn(tierKeeper Keeper, sk GovStakingKeeper, ac addresscodec.Codec) govkeeper.CalculateVoteResultsAndVotingPowerFn {
	return func(
		ctx context.Context,
		k govkeeper.Keeper,
		proposal v1.Proposal,
		validators map[string]v1.ValidatorGovInfo,
	) (math.LegacyDec, map[v1.VoteOption]math.LegacyDec, error) {
		totalVotingPower := math.LegacyZeroDec()

		results := make(map[v1.VoteOption]math.LegacyDec)
		results[v1.OptionYes] = math.LegacyZeroDec()
		results[v1.OptionAbstain] = math.LegacyZeroDec()
		results[v1.OptionNo] = math.LegacyZeroDec()
		results[v1.OptionNoWithVeto] = math.LegacyZeroDec()

		rng := collections.NewPrefixedPairRange[uint64, sdk.AccAddress](proposal.Id)
		votesToRemove := []collections.Pair[uint64, sdk.AccAddress]{}
		err := k.Votes.Walk(ctx, rng, func(key collections.Pair[uint64, sdk.AccAddress], vote v1.Vote) (bool, error) {
			// Resolve voter bytes using the address codec (L-1 fix).
			voterBytes, err := ac.StringToBytes(vote.Voter)
			if err != nil {
				return false, err
			}
			voter := sdk.AccAddress(voterBytes)

			// Check if voter is a validator.
			valAddrStr, err := sk.ValidatorAddressCodec().BytesToString(voter)
			if err != nil {
				return false, err
			}
			if val, ok := validators[valAddrStr]; ok {
				val.Vote = vote.Options
				validators[valAddrStr] = val
			}

			// Iterate staking delegations.
			err = sk.IterateDelegations(ctx, voter, func(_ int64, delegation stakingtypes.DelegationI) (stop bool) {
				valAddrStr := delegation.GetValidatorAddr()
				if val, ok := validators[valAddrStr]; ok {
					if val.DelegatorShares.IsZero() {
						return false // defensive: avoid division by zero
					}
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

			// Add tier voting power for this voter and deduct the corresponding
			// shares from validators in a single pass. We compute tier voting power
			// per-position using the same shares-to-tokens formula as staking
			// delegations (DelegatedShares * BondedTokens / DelegatorShares).
			// This ensures the tier power addition and DelegatorDeductions are
			// perfectly symmetric: power shifts from the validator's residual to
			// the voter's explicit vote with zero net effect on totalVotingPower.
			// Only positions on bonded validators are counted — positions on
			// non-bonded validators have no active delegation to deduct from, so
			// counting them would inflate totalVotingPower and quorum.
			positions, err := tierKeeper.GetPositionsByOwner(ctx, vote.Voter)
			if err != nil {
				return false, fmt.Errorf("failed to get tier positions for %s: %w", vote.Voter, err)
			}
			for _, pos := range positions {
				if pos.Validator == "" || pos.IsUnbonding || pos.DelegatedShares.IsNil() || !pos.DelegatedShares.IsPositive() {
					continue
				}
				val, ok := validators[pos.Validator]
				if !ok {
					continue // validator not in bonded set — skip to avoid inflation
				}
				if val.DelegatorShares.IsZero() {
					continue // defensive: avoid division by zero (should never happen for bonded validators)
				}

				// Compute token-equivalent power using same formula as staking delegations.
				tierVotingPower := pos.DelegatedShares.MulInt(val.BondedTokens).Quo(val.DelegatorShares)
				for _, option := range vote.Options {
					weight, _ := math.LegacyNewDecFromStr(option.Weight)
					subPower := tierVotingPower.Mul(weight)
					results[option.Option] = results[option.Option].Add(subPower)
				}
				totalVotingPower = totalVotingPower.Add(tierVotingPower)

				// Deduct shares from validator to prevent double-counting.
				val.DelegatorDeductions = val.DelegatorDeductions.Add(pos.DelegatedShares)
				validators[pos.Validator] = val
			}

			votesToRemove = append(votesToRemove, key)
			return false, nil
		})
		if err != nil {
			return math.LegacyZeroDec(), nil, fmt.Errorf("error while iterating delegations: %w", err)
		}

		// Remove processed votes.
		for _, key := range votesToRemove {
			if err := k.Votes.Remove(ctx, key); err != nil {
				return math.LegacyDec{}, nil, fmt.Errorf("error while removing vote (%d/%s): %w", key.K1(), key.K2(), err)
			}
		}

		// Tally remaining validator votes.
		for _, val := range validators {
			if len(val.Vote) == 0 {
				continue
			}
			if val.DelegatorShares.IsZero() {
				continue // defensive: avoid division by zero
			}
			// NOTE: Unlike the default SDK tally, tier position shares are added
			// to DelegatorDeductions (line 124), which can cause deductions to
			// exceed DelegatorShares after a slash reduces the validator's total
			// shares. Clamp to zero to prevent negative voting power.
			sharesAfterDeductions := val.DelegatorShares.Sub(val.DelegatorDeductions)
			if sharesAfterDeductions.IsNegative() {
				sharesAfterDeductions = math.LegacyZeroDec()
			}
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
