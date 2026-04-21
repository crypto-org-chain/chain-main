package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from genesis.
// SetPosition rebuilds all secondary indexes, so derived data does not need
// to be stored in genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState) {
	// Materialize module accounts during chain init so direct sends cannot create
	// a plain base account at either module address before the first message.
	if k.accountKeeper.GetModuleAccount(ctx, types.ModuleName) == nil {
		panic("tieredrewards module account was not created")
	}
	if k.accountKeeper.GetModuleAccount(ctx, types.RewardsPoolName) == nil {
		panic("tieredrewards rewards pool module account was not created")
	}

	if err := k.SetParams(ctx, data.Params); err != nil {
		panic(err)
	}

	for _, tier := range data.Tiers {
		if err := k.setTier(ctx, tier); err != nil {
			panic(err)
		}
	}

	for _, pos := range data.Positions {
		if err := k.setPosition(ctx, pos); err != nil {
			panic(err)
		}
	}

	// Set sequence after positions to avoid interference with SetPosition's increasePositionCount.
	if data.NextPositionId > 0 {
		if err := k.NextPositionId.Set(ctx, data.NextPositionId); err != nil {
			panic(err)
		}
	}

	for _, entry := range data.ValidatorRewardRatios {
		valAddr, err := sdk.ValAddressFromBech32(entry.Validator)
		if err != nil {
			panic(err)
		}
		if err := k.ValidatorRewardRatio.Set(ctx, valAddr, entry.RewardRatio); err != nil {
			panic(err)
		}
	}

	for _, mapping := range data.UnbondingDelegationMappings {
		if err := k.setUnbondingPositionMapping(ctx, mapping.UnbondingId, mapping.PositionId); err != nil {
			panic(err)
		}
	}

	for _, mapping := range data.RedelegationMappings {
		if err := k.setRedelegationPositionMapping(ctx, mapping.UnbondingId, mapping.PositionId); err != nil {
			panic(err)
		}
	}

	for _, checkpoint := range data.ValidatorBonusPauseCheckpoints {
		valAddr, err := sdk.ValAddressFromBech32(checkpoint.Validator)
		if err != nil {
			panic(err)
		}
		if err := k.setValidatorBonusPauseAtUnix(ctx, valAddr, checkpoint.UnixTime); err != nil {
			panic(err)
		}
	}

	for _, checkpoint := range data.ValidatorBonusResumeCheckpoints {
		valAddr, err := sdk.ValAddressFromBech32(checkpoint.Validator)
		if err != nil {
			panic(err)
		}
		if err := k.setValidatorBonusResumeAtUnix(ctx, valAddr, checkpoint.UnixTime); err != nil {
			panic(err)
		}
	}

	for _, rate := range data.ValidatorBonusPauseRates {
		valAddr, err := sdk.ValAddressFromBech32(rate.Validator)
		if err != nil {
			panic(err)
		}
		decRate, err := sdkmath.LegacyNewDecFromStr(rate.TokensPerShare)
		if err != nil {
			panic(err)
		}
		if err := k.setValidatorBonusPauseRate(ctx, valAddr, decRate); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.Params.Get(ctx)
	if err != nil {
		panic(err)
	}

	var tiers []types.Tier
	err = k.Tiers.Walk(ctx, nil, func(_ uint32, tier types.Tier) (bool, error) {
		tiers = append(tiers, tier)
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var positions []types.Position
	err = k.Positions.Walk(ctx, nil, func(positionID uint64, _ types.Position) (bool, error) {
		pos, err := k.getPosition(ctx, positionID)
		if err != nil {
			return false, err
		}
		positions = append(positions, pos)
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	nextPositionId, err := k.NextPositionId.Peek(ctx)
	if err != nil {
		panic(err)
	}

	var validatorRewardRatios []types.ValidatorRewardRatioEntry
	err = k.ValidatorRewardRatio.Walk(ctx, nil, func(valAddr sdk.ValAddress, ratio types.ValidatorRewardRatio) (bool, error) {
		validatorRewardRatios = append(validatorRewardRatios, types.ValidatorRewardRatioEntry{
			Validator:   valAddr.String(),
			RewardRatio: ratio,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var unbondingDelegationMappings []types.UnbondingMapping
	err = k.UnbondingDelegationMappings.Walk(ctx, nil, func(unbondingId, positionId uint64) (bool, error) {
		unbondingDelegationMappings = append(unbondingDelegationMappings, types.UnbondingMapping{
			UnbondingId: unbondingId,
			PositionId:  positionId,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var redelegationMappings []types.UnbondingMapping
	err = k.RedelegationMappings.Walk(ctx, nil, func(unbondingId, positionId uint64) (bool, error) {
		redelegationMappings = append(redelegationMappings, types.UnbondingMapping{
			UnbondingId: unbondingId,
			PositionId:  positionId,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var validatorBonusPauseCheckpoints []types.ValidatorBonusCheckpointEntry
	err = k.ValidatorBonusPauseAt.Walk(ctx, nil, func(valAddr sdk.ValAddress, unixTime int64) (bool, error) {
		validatorBonusPauseCheckpoints = append(validatorBonusPauseCheckpoints, types.ValidatorBonusCheckpointEntry{
			Validator: valAddr.String(),
			UnixTime:  unixTime,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var validatorBonusResumeCheckpoints []types.ValidatorBonusCheckpointEntry
	err = k.ValidatorBonusResumeAt.Walk(ctx, nil, func(valAddr sdk.ValAddress, unixTime int64) (bool, error) {
		validatorBonusResumeCheckpoints = append(validatorBonusResumeCheckpoints, types.ValidatorBonusCheckpointEntry{
			Validator: valAddr.String(),
			UnixTime:  unixTime,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var validatorBonusPauseRates []types.ValidatorBonusRateEntry
	err = k.ValidatorBonusPauseRate.Walk(ctx, nil, func(valAddr sdk.ValAddress, tokensPerShare sdkmath.LegacyDec) (bool, error) {
		validatorBonusPauseRates = append(validatorBonusPauseRates, types.ValidatorBonusRateEntry{
			Validator:      valAddr.String(),
			TokensPerShare: tokensPerShare.String(),
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	return &types.GenesisState{
		Params:                          params,
		Tiers:                           tiers,
		Positions:                       positions,
		NextPositionId:                  nextPositionId,
		ValidatorRewardRatios:           validatorRewardRatios,
		UnbondingDelegationMappings:     unbondingDelegationMappings,
		RedelegationMappings:            redelegationMappings,
		ValidatorBonusPauseCheckpoints:  validatorBonusPauseCheckpoints,
		ValidatorBonusResumeCheckpoints: validatorBonusResumeCheckpoints,
		ValidatorBonusPauseRates:        validatorBonusPauseRates,
	}
}
