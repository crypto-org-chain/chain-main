package keeper

import (
	"fmt"

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

	currentSeqByValidator := make(map[string]uint64)
	for _, entry := range data.ValidatorEventSeqs {
		currentSeqByValidator[entry.Validator] = entry.CurrentSeq
	}

	// delegatedPosSeqsByVal tracks, per validator, the LastEventSeq of each
	// delegated position so we can cross-validate event ReferenceCounts below.
	delegatedPosSeqsByVal := make(map[string][]uint64)

	for _, pos := range data.Positions {
		del, err := k.getDelegation(ctx, pos.Id)
		if err != nil {
			panic(fmt.Errorf("failed to get delegation for position %d: %w", pos.Id, err))
		}

		posState := types.PositionState{Position: pos, Delegation: del}

		if del != nil {
			validator := del.ValidatorAddress
			if pos.LastEventSeq > currentSeqByValidator[validator] {
				panic(fmt.Errorf(
					"position %d has LastEventSeq (%d) greater than validator %s current_seq (%d)",
					pos.Id, pos.LastEventSeq, validator, currentSeqByValidator[validator],
				))
			}
			delegatedPosSeqsByVal[validator] = append(delegatedPosSeqsByVal[validator], pos.LastEventSeq)
		}

		if err := posState.Validate(); err != nil {
			panic(fmt.Errorf("position state validation failed for position %d: %w", pos.Id, err))
		}

		if err := k.setPosition(ctx, pos, &ValidatorTransition{PreviousAddress: ""}); err != nil {
			panic(err)
		}
	}

	// Set sequence after positions to avoid interference with SetPosition's increasePositionCount.
	if data.NextPositionId > 0 {
		if err := k.NextPositionId.Set(ctx, data.NextPositionId); err != nil {
			panic(err)
		}
	}

	for _, entry := range data.RedelegationMappings {
		red, err := k.stakingKeeper.GetRedelegationByUnbondingID(ctx, entry.UnbondingId)
		if err != nil {
			panic(fmt.Errorf(
				"redelegation mapping (unbonding_id=%d, position_id=%d) has no matching staking redelegation: %w",
				entry.UnbondingId, entry.PositionId, err,
			))
		}
		expectedDelAddr := types.GetDelegatorAddress(entry.PositionId).String()
		if red.DelegatorAddress != expectedDelAddr {
			panic(fmt.Errorf(
				"redelegation mapping (unbonding_id=%d, position_id=%d) delegator mismatch: staking has %q, expected %q",
				entry.UnbondingId, entry.PositionId, red.DelegatorAddress, expectedDelAddr,
			))
		}
		if err := k.setRedelegationMapping(ctx, entry.UnbondingId, entry.PositionId); err != nil {
			panic(err)
		}
	}

	for _, entry := range data.ValidatorEvents {
		// Cross-validate: ReferenceCount must equal the number of delegated
		// positions on this validator with LastEventSeq < event.Sequence. Too
		// high and the event is never garbage-collected (storage leak); too low
		// and it is prematurely collected, causing unprocessed positions to
		// skip the segment (under/over payment).
		var expected uint64
		for _, lastSeq := range delegatedPosSeqsByVal[entry.Validator] {
			if lastSeq < entry.Sequence {
				expected++
			}
		}
		if entry.Event.ReferenceCount != expected {
			panic(fmt.Errorf(
				"validator %s event seq %d has ReferenceCount %d but %d positions would process it",
				entry.Validator, entry.Sequence, entry.Event.ReferenceCount, expected,
			))
		}
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

	var redelegationMappings []types.RedelegationMapping
	err = k.RedelegationMappings.Walk(ctx, nil, func(unbondingId, positionId uint64) (bool, error) {
		redelegationMappings = append(redelegationMappings, types.RedelegationMapping{
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
		Params:               params,
		Tiers:                tiers,
		Positions:            positions,
		NextPositionId:       nextPositionId,
		RedelegationMappings: redelegationMappings,
		ValidatorEvents:      validatorEvents,
		ValidatorEventSeqs:   validatorEventSeqs,
	}
}
