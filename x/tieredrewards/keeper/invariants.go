package keeper

import (
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterInvariants registers all tieredrewards module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "module-balance", ModuleBalanceInvariant(k))
	ir.RegisterRoute(types.ModuleName, "position-consistency", PositionConsistencyInvariant(k))
	ir.RegisterRoute(types.ModuleName, "total-tier-shares", TotalTierSharesInvariant(k))
	ir.RegisterRoute(types.ModuleName, "unbonding-consistency", UnbondingConsistencyInvariant(k))
}

// ModuleBalanceInvariant checks that the module account holds enough tokens
// to cover all locked positions and accumulated pending base rewards.
func ModuleBalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		bondDenom, err := k.stakingKeeper.BondDenom(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "module-balance",
				fmt.Sprintf("failed to get bond denom: %s", err)), true
		}

		totalLocked := math.ZeroInt()
		totalPendingRewards := sdk.NewCoins()
		positions, err := k.GetAllPositions(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "module-balance",
				fmt.Sprintf("failed to get positions: %s", err)), true
		}
		for _, pos := range positions {
			if !pos.AmountLocked.IsNil() && pos.AmountLocked.IsPositive() {
				// Only count positions whose tokens are in the module account.
				// Delegated and unbonding tokens are held by the staking module.
				if pos.Validator == "" && !pos.IsUnbonding {
					totalLocked = totalLocked.Add(pos.AmountLocked)
				}
			}
			// PendingBaseRewards are always held in the module account regardless
			// of delegation status.
			if pos.PendingBaseRewards.IsAllPositive() {
				totalPendingRewards = totalPendingRewards.Add(pos.PendingBaseRewards...)
			}
		}

		moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)

		// Check bond denom: must cover locked + pending rewards in bond denom.
		totalNeeded := totalLocked.Add(totalPendingRewards.AmountOf(bondDenom))
		moduleBalance := k.bankKeeper.GetBalance(ctx, moduleAddr, bondDenom)

		broken := moduleBalance.Amount.LT(totalNeeded)
		return sdk.FormatInvariant(types.ModuleName, "module-balance",
			fmt.Sprintf("module balance %s < total needed %s (locked %s + pending %s): %t",
				moduleBalance.Amount, totalNeeded, totalLocked, totalPendingRewards.AmountOf(bondDenom), broken)), broken
	}
}

// PositionConsistencyInvariant checks that every delegated, non-unbonding
// position has positive shares, and that every unbonding position has zero shares.
func PositionConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		positions, err := k.GetAllPositions(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "position-consistency",
				fmt.Sprintf("failed to get positions: %s", err)), true
		}

		for _, pos := range positions {
			if pos.Validator != "" && !pos.IsUnbonding {
				if pos.DelegatedShares.IsNil() || !pos.DelegatedShares.IsPositive() {
					return sdk.FormatInvariant(types.ModuleName, "position-consistency",
						fmt.Sprintf("position %d is delegated to %s but has non-positive shares: %s",
							pos.PositionId, pos.Validator, pos.DelegatedShares)), true
				}
			}
			// Unbonding positions must have exactly zero DelegatedShares (set in TierUndelegate).
			if pos.IsUnbonding && !pos.DelegatedShares.IsNil() && !pos.DelegatedShares.IsZero() {
				return sdk.FormatInvariant(types.ModuleName, "position-consistency",
					fmt.Sprintf("position %d is unbonding but has non-zero shares: %s",
						pos.PositionId, pos.DelegatedShares)), true
			}
		}

		return sdk.FormatInvariant(types.ModuleName, "position-consistency",
			"all positions have consistent delegation state"), false
	}
}

// TotalTierSharesInvariant checks that TotalTierShares[validator] equals the
// sum of DelegatedShares across all actively delegated positions for that validator.
func TotalTierSharesInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		positions, err := k.GetAllPositions(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "total-tier-shares",
				fmt.Sprintf("failed to get positions: %s", err)), true
		}

		// Compute expected totals from positions.
		expected := make(map[string]math.LegacyDec)
		for _, pos := range positions {
			if pos.Validator != "" && !pos.IsUnbonding && !pos.DelegatedShares.IsNil() && pos.DelegatedShares.IsPositive() {
				cur, ok := expected[pos.Validator]
				if !ok {
					cur = math.LegacyZeroDec()
				}
				expected[pos.Validator] = cur.Add(pos.DelegatedShares)
			}
		}

		// Compare against stored totals (forward direction: positions → stored).
		for val, exp := range expected {
			stored, err := k.GetTotalTierShares(ctx, val)
			if err != nil {
				return sdk.FormatInvariant(types.ModuleName, "total-tier-shares",
					fmt.Sprintf("failed to get stored total for %s: %s", val, err)), true
			}
			if !stored.Equal(exp) {
				return sdk.FormatInvariant(types.ModuleName, "total-tier-shares",
					fmt.Sprintf("validator %s: stored total %s != computed total %s",
						val, stored, exp)), true
			}
		}

		// Reverse check: detect orphaned entries in TotalTierShares with no matching positions.
		err = k.TotalTierShares.Walk(ctx, nil, func(validator string, stored math.LegacyDec) (bool, error) {
			if _, ok := expected[validator]; !ok {
				return true, fmt.Errorf("validator %s has stored total %s but no active positions", validator, stored)
			}
			return false, nil
		})
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "total-tier-shares",
				fmt.Sprintf("orphaned TotalTierShares entry: %s", err)), true
		}

		return sdk.FormatInvariant(types.ModuleName, "total-tier-shares",
			"all validator tier share totals are consistent"), false
	}
}

// UnbondingConsistencyInvariant checks that every position with IsUnbonding==true
// has a corresponding entry in UnbondingPositions, and vice versa.
func UnbondingConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		positions, err := k.GetAllPositions(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "unbonding-consistency",
				fmt.Sprintf("failed to get positions: %s", err)), true
		}

		// Forward: every unbonding position must be in the UnbondingPositions map.
		for _, pos := range positions {
			if !pos.IsUnbonding {
				continue
			}
			has, err := k.UnbondingPositions.Has(ctx, pos.PositionId)
			if err != nil {
				return sdk.FormatInvariant(types.ModuleName, "unbonding-consistency",
					fmt.Sprintf("failed to check UnbondingPositions for %d: %s", pos.PositionId, err)), true
			}
			if !has {
				return sdk.FormatInvariant(types.ModuleName, "unbonding-consistency",
					fmt.Sprintf("position %d has IsUnbonding=true but no UnbondingPositions entry", pos.PositionId)), true
			}
		}

		// Reverse: every entry in UnbondingPositions must reference a position with IsUnbonding==true.
		err = k.UnbondingPositions.Walk(ctx, nil, func(posId uint64, _ int64) (bool, error) {
			pos, posErr := k.GetPosition(ctx, posId)
			if posErr != nil {
				return true, fmt.Errorf("UnbondingPositions entry %d references non-existent position: %s", posId, posErr)
			}
			if !pos.IsUnbonding {
				return true, fmt.Errorf("UnbondingPositions entry %d but position has IsUnbonding=false", posId)
			}
			return false, nil
		})
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "unbonding-consistency",
				fmt.Sprintf("orphaned UnbondingPositions entry: %s", err)), true
		}

		return sdk.FormatInvariant(types.ModuleName, "unbonding-consistency",
			"all unbonding positions are consistently tracked"), false
	}
}
