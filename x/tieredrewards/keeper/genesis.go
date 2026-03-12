package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState) {
	if err := k.Params.Set(ctx, data.Params); err != nil {
		panic(err)
	}

	// Set next position ID directly.
	if err := k.NextPositionID.Set(ctx, data.NextPositionId); err != nil {
		panic(err)
	}

	// Restore positions and rebuild TotalTierShares and UnbondingPositions.
	totals := make(map[string]math.LegacyDec)
	for _, pos := range data.Positions {
		if err := k.Positions.Set(ctx, pos.PositionId, pos); err != nil {
			panic(err)
		}
		if pos.Validator != "" && !pos.IsUnbonding && !pos.DelegatedShares.IsNil() && pos.DelegatedShares.IsPositive() {
			cur, ok := totals[pos.Validator]
			if !ok {
				cur = math.LegacyZeroDec()
			}
			totals[pos.Validator] = cur.Add(pos.DelegatedShares)
		}
		if pos.IsUnbonding && !pos.UnbondingCompletionTime.IsZero() {
			if err := k.UnbondingPositions.Set(ctx, pos.PositionId, pos.UnbondingCompletionTime.Unix()); err != nil {
				panic(err)
			}
		}
	}
	for val, total := range totals {
		if err := k.TotalTierShares.Set(ctx, val, total); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.Params.Get(ctx)
	if err != nil {
		panic(err)
	}

	positions, err := k.GetAllPositions(ctx)
	if err != nil {
		panic(err)
	}

	nextId, err := k.NextPositionID.Peek(ctx)
	if err != nil {
		panic(err)
	}

	return &types.GenesisState{
		Params:         params,
		Positions:      positions,
		NextPositionId: nextId,
	}
}
