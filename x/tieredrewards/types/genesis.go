package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

func ValidateGenesis(data GenesisState) error {
	if err := data.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	tierIDs := make(map[uint32]struct{}, len(data.Tiers))
	for i, tier := range data.Tiers {
		if err := tier.Validate(); err != nil {
			return fmt.Errorf("invalid tier at index %d: %w", i, err)
		}
		if _, dup := tierIDs[tier.Id]; dup {
			return fmt.Errorf("duplicate tier ID %d at index %d", tier.Id, i)
		}
		tierIDs[tier.Id] = struct{}{}
	}

	posIDs := make(map[uint64]struct{}, len(data.Positions))
	var maxPosID uint64
	for i, pos := range data.Positions {
		if err := pos.Validate(); err != nil {
			return fmt.Errorf("invalid position at index %d: %w", i, err)
		}
		if _, dup := posIDs[pos.Id]; dup {
			return fmt.Errorf("duplicate position ID %d at index %d", pos.Id, i)
		}
		posIDs[pos.Id] = struct{}{}

		if _, ok := tierIDs[pos.TierId]; !ok {
			return fmt.Errorf("position %d references unknown tier ID %d", pos.Id, pos.TierId)
		}

		if pos.Id > maxPosID {
			maxPosID = pos.Id
		}
	}

	if len(data.Positions) > 0 && data.NextPositionId <= maxPosID {
		return fmt.Errorf("next_position_id (%d) must be greater than the highest position ID (%d)", data.NextPositionId, maxPosID)
	}

	seenValidators := make(map[string]struct{}, len(data.ValidatorRewardRatios))
	for i, entry := range data.ValidatorRewardRatios {
		if _, err := sdk.ValAddressFromBech32(entry.Validator); err != nil {
			return fmt.Errorf("invalid validator address in reward ratio at index %d: %w", i, err)
		}
		if err := entry.RewardRatio.CumulativeRewardsPerShare.Validate(); err != nil {
			return fmt.Errorf("invalid reward ratio payload at index %d: %w", i, err)
		}
		if _, dup := seenValidators[entry.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in reward ratios at index %d", entry.Validator, i)
		}
		seenValidators[entry.Validator] = struct{}{}
	}

	seenUnbondingIDs := make(map[uint64]struct{}, len(data.UnbondingDelegationMappings))
	for i, mapping := range data.UnbondingDelegationMappings {
		if _, dup := seenUnbondingIDs[mapping.UnbondingId]; dup {
			return fmt.Errorf("duplicate unbonding ID %d at index %d", mapping.UnbondingId, i)
		}
		seenUnbondingIDs[mapping.UnbondingId] = struct{}{}

		if _, ok := posIDs[mapping.PositionId]; !ok {
			return fmt.Errorf("unbonding mapping at index %d references unknown position ID %d", i, mapping.PositionId)
		}
	}
	// redelegation unbonding ids share the same global counter as unbonding delegation ids, so there should be no duplicates.
	for i, mapping := range data.RedelegationMappings {
		if _, dup := seenUnbondingIDs[mapping.UnbondingId]; dup {
			return fmt.Errorf("duplicate redelegation ID %d at index %d", mapping.UnbondingId, i)
		}
		seenUnbondingIDs[mapping.UnbondingId] = struct{}{}

		if _, ok := posIDs[mapping.PositionId]; !ok {
			return fmt.Errorf("redelegation mapping at index %d references unknown position ID %d", i, mapping.PositionId)
		}
	}

	seenPauseValidators := make(map[string]struct{}, len(data.ValidatorBonusPauseCheckpoints))
	for i, checkpoint := range data.ValidatorBonusPauseCheckpoints {
		if _, err := sdk.ValAddressFromBech32(checkpoint.Validator); err != nil {
			return fmt.Errorf("invalid validator address in bonus pause checkpoint at index %d: %w", i, err)
		}
		if checkpoint.UnixTime < 0 {
			return fmt.Errorf("negative pause unix_time at index %d", i)
		}
		if _, dup := seenPauseValidators[checkpoint.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in bonus pause checkpoints at index %d", checkpoint.Validator, i)
		}
		seenPauseValidators[checkpoint.Validator] = struct{}{}
	}

	seenResumeValidators := make(map[string]struct{}, len(data.ValidatorBonusResumeCheckpoints))
	for i, checkpoint := range data.ValidatorBonusResumeCheckpoints {
		if _, err := sdk.ValAddressFromBech32(checkpoint.Validator); err != nil {
			return fmt.Errorf("invalid validator address in bonus resume checkpoint at index %d: %w", i, err)
		}
		if checkpoint.UnixTime < 0 {
			return fmt.Errorf("negative resume unix_time at index %d", i)
		}
		if _, dup := seenResumeValidators[checkpoint.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in bonus resume checkpoints at index %d", checkpoint.Validator, i)
		}
		seenResumeValidators[checkpoint.Validator] = struct{}{}
	}
	for validator := range seenResumeValidators {
		if _, ok := seenPauseValidators[validator]; !ok {
			return fmt.Errorf("bonus resume checkpoint without pause checkpoint for validator %s", validator)
		}
	}

	seenPauseRateValidators := make(map[string]struct{}, len(data.ValidatorBonusPauseRates))
	for i, rate := range data.ValidatorBonusPauseRates {
		if _, err := sdk.ValAddressFromBech32(rate.Validator); err != nil {
			return fmt.Errorf("invalid validator address in bonus pause rate at index %d: %w", i, err)
		}
		decRate, err := sdkmath.LegacyNewDecFromStr(rate.TokensPerShare)
		if err != nil {
			return fmt.Errorf("invalid tokens_per_share in bonus pause rate at index %d: %w", i, err)
		}
		if decRate.IsNegative() {
			return fmt.Errorf("negative tokens_per_share in bonus pause rate at index %d", i)
		}
		if _, dup := seenPauseRateValidators[rate.Validator]; dup {
			return fmt.Errorf("duplicate validator %s in bonus pause rates at index %d", rate.Validator, i)
		}
		seenPauseRateValidators[rate.Validator] = struct{}{}
	}

	for validator := range seenPauseValidators {
		if _, ok := seenPauseRateValidators[validator]; !ok {
			return fmt.Errorf("missing bonus pause rate for validator %s", validator)
		}
	}
	for validator := range seenPauseRateValidators {
		if _, ok := seenPauseValidators[validator]; !ok {
			return fmt.Errorf("bonus pause rate without pause checkpoint for validator %s", validator)
		}
	}

	return nil
}
