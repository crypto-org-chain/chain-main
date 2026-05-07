package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from genesis.
// SetPosition rebuilds all secondary indexes, so derived data does not need
// to be stored in genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState) {
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
		if err := k.SetTier(ctx, tier); err != nil {
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

	for _, entry := range data.ValidatorEvents {
		valAddr, err := sdk.ValAddressFromBech32(entry.Validator)
		if err != nil {
			panic(err)
		}
		if err := k.ValidatorEvents.Set(ctx, collections.Join(valAddr, entry.Sequence), entry.Event); err != nil {
			panic(err)
		}
	}

	for _, entry := range data.ValidatorEventSeqs {
		valAddr, err := sdk.ValAddressFromBech32(entry.Validator)
		if err != nil {
			panic(err)
		}
		if err := k.ValidatorEventSeq.Set(ctx, valAddr, entry.CurrentSeq); err != nil {
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

	var validatorEvents []types.ValidatorEventEntry
	err = k.ValidatorEvents.Walk(ctx, nil, func(key collections.Pair[sdk.ValAddress, uint64], event types.ValidatorEvent) (bool, error) {
		validatorEvents = append(validatorEvents, types.ValidatorEventEntry{
			Validator: key.K1().String(),
			Sequence:  key.K2(),
			Event:     event,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	var validatorEventSeqs []types.ValidatorEventSeqEntry
	err = k.ValidatorEventSeq.Walk(ctx, nil, func(valAddr sdk.ValAddress, currentSeq uint64) (bool, error) {
		validatorEventSeqs = append(validatorEventSeqs, types.ValidatorEventSeqEntry{
			Validator:  valAddr.String(),
			CurrentSeq: currentSeq,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}

	return &types.GenesisState{
		Params:                      params,
		Tiers:                       tiers,
		Positions:                   positions,
		NextPositionId:              nextPositionId,
		UnbondingDelegationMappings: unbondingDelegationMappings,
		RedelegationMappings:        redelegationMappings,
		ValidatorEvents:             validatorEvents,
		ValidatorEventSeqs:          validatorEventSeqs,
	}
}
