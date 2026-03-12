package keeper

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate2to3 migrates from consensus version 2 to 3.
// It rebuilds the PositionsByOwner and PositionsByValidator secondary indexes
// and recomputes TotalTierShares from position data.
func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	// Clear any pre-existing TotalTierShares entries for idempotent recomputation.
	if err := m.keeper.TotalTierShares.Clear(ctx, nil); err != nil {
		return err
	}

	// Collect all positions first to avoid modifying the store during iteration.
	var positions []types.TierPosition
	err := m.keeper.Positions.Walk(ctx, nil, func(id uint64, pos types.TierPosition) (bool, error) {
		positions = append(positions, pos)
		return false, nil
	})
	if err != nil {
		return err
	}

	// Re-set each position to trigger index Reference() calls,
	// which builds the owner and validator secondary indexes.
	totals := make(map[string]math.LegacyDec)
	for _, pos := range positions {
		if err := m.keeper.Positions.Set(ctx, pos.PositionId, pos); err != nil {
			return err
		}
		if pos.Validator != "" && !pos.IsUnbonding && !pos.DelegatedShares.IsNil() && pos.DelegatedShares.IsPositive() {
			cur, ok := totals[pos.Validator]
			if !ok {
				cur = math.LegacyZeroDec()
			}
			totals[pos.Validator] = cur.Add(pos.DelegatedShares)
		}
	}

	// Write TotalTierShares.
	for val, total := range totals {
		if err := m.keeper.TotalTierShares.Set(ctx, val, total); err != nil {
			return err
		}
	}

	return nil
}

// Migrate3to4 migrates from consensus version 3 to 4.
// It rebuilds the UnbondingPositions map from existing positions that are
// currently unbonding, enabling efficient EndBlocker lookups.
func (m Migrator) Migrate3to4(ctx sdk.Context) error {
	// Clear any pre-existing UnbondingPositions entries for idempotent recomputation.
	if err := m.keeper.UnbondingPositions.Clear(ctx, nil); err != nil {
		return err
	}

	var positions []types.TierPosition
	err := m.keeper.Positions.Walk(ctx, nil, func(id uint64, pos types.TierPosition) (bool, error) {
		positions = append(positions, pos)
		return false, nil
	})
	if err != nil {
		return err
	}

	for _, pos := range positions {
		if pos.IsUnbonding && !pos.UnbondingCompletionTime.IsZero() {
			if err := m.keeper.UnbondingPositions.Set(ctx, pos.PositionId, pos.UnbondingCompletionTime.Unix()); err != nil {
				return err
			}
		}
	}

	return nil
}
