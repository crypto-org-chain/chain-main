package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
// Order matters: params → tiers → positions → sequence → reward ratios → unbonding mappings.
// SetPosition rebuilds all secondary indexes (PositionsByOwner, PositionsByTier,
// PositionsByValidator) and PositionCountByTier, so derived data does not need
// to be stored in genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState) {
	if err := k.SetParams(ctx, data.Params); err != nil {
		panic(err)
	}

	for _, tier := range data.Tiers {
		if err := k.SetTier(ctx, tier); err != nil {
			panic(err)
		}
	}

	for _, pos := range data.Positions {
		if err := k.SetPosition(ctx, pos); err != nil {
			panic(err)
		}
	}

	// Set sequence after positions to avoid interference with SetPosition's
	// increasePositionCount. The sequence only tracks the next ID to assign.
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

	for _, mapping := range data.UnbondingMappings {
		if err := k.UnbondingIdToPositionId.Set(ctx, mapping.UnbondingId, mapping.PositionId); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper.
// Only primary data is exported; secondary indexes are rebuilt on InitGenesis.
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
	err = k.Positions.Walk(ctx, nil, func(_ uint64, pos types.Position) (bool, error) {
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

	var unbondingMappings []types.UnbondingMapping
	err = k.UnbondingIdToPositionId.Walk(ctx, nil, func(unbondingId, positionId uint64) (bool, error) {
		unbondingMappings = append(unbondingMappings, types.UnbondingMapping{
			UnbondingId: unbondingId,
			PositionId:  positionId,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	return &types.GenesisState{
		Params:                params,
		Tiers:                 tiers,
		Positions:             positions,
		NextPositionId:        nextPositionId,
		ValidatorRewardRatios: validatorRewardRatios,
		UnbondingMappings:     unbondingMappings,
	}
}
