package keeper

import (
	"context"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Hooks wraps the Keeper to implement the staking hooks interface.
type Hooks struct {
	k Keeper
}

var _ stakingtypes.StakingHooks = Hooks{}

// Hooks returns a Hooks wrapper around the Keeper.
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// BeforeValidatorSlashed iterates all tier positions delegated to the slashed
// validator and reduces their AmountLocked by the slash fraction.
func (h Hooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error {
	valAddrStr := valAddr.String()

	// Use the validator index for efficient lookup instead of iterating all positions.
	iter, err := h.k.Positions.Indexes.Validator.MatchExact(ctx, valAddrStr)
	if err != nil {
		return err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		pk, err := iter.PrimaryKey()
		if err != nil {
			return err
		}
		pos, err := h.k.Positions.Get(ctx, pk)
		if err != nil {
			return err
		}

		// Reduce AmountLocked by the slash fraction for BOTH active and unbonding
		// positions. The staking module applies the same fraction to bonded tokens
		// AND unbonding delegations (for entries created at/after the infraction
		// height). Without this reduction, unbonding positions would track a stale
		// AmountLocked that exceeds the actual tokens the staking module will return,
		// making the position permanently unwithdrawable.
		//
		// Edge case: if unbonding started BEFORE the infraction height, the staking
		// module skips slashing that unbonding entry but our hook still reduces
		// AmountLocked (slight over-reduction by the slash fraction). This is
		// acceptable — the user loses only the slash fraction (typically 5%), whereas
		// NOT reducing would make the position permanently stuck.
		slashAmount := math.LegacyNewDecFromInt(pos.AmountLocked).Mul(fraction).TruncateInt()
		pos.AmountLocked = pos.AmountLocked.Sub(slashAmount)
		if pos.AmountLocked.IsNegative() {
			pos.AmountLocked = math.ZeroInt()
		}

		// NOTE: DelegatedShares are NOT reduced on slash. In standard Cosmos SDK,
		// slashing reduces the validator's Tokens but does NOT change DelegatorShares.
		// The exchange rate (tokens/share) changes instead. Reducing shares here would
		// create a mismatch with the actual staking delegation. TotalTierShares is also
		// left unchanged since it tracks delegation shares, not token amounts.

		if err := h.k.SetPosition(ctx, pos); err != nil {
			return err
		}
	}
	return nil
}

// AfterValidatorCreated implements StakingHooks.
func (h Hooks) AfterValidatorCreated(_ context.Context, _ sdk.ValAddress) error { return nil }

// BeforeValidatorModified implements StakingHooks.
func (h Hooks) BeforeValidatorModified(_ context.Context, _ sdk.ValAddress) error { return nil }

// AfterValidatorRemoved implements StakingHooks.
func (h Hooks) AfterValidatorRemoved(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

// AfterValidatorBonded implements StakingHooks.
func (h Hooks) AfterValidatorBonded(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

// AfterValidatorBeginUnbonding implements StakingHooks.
func (h Hooks) AfterValidatorBeginUnbonding(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

// BeforeDelegationCreated implements StakingHooks.
func (h Hooks) BeforeDelegationCreated(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

// BeforeDelegationSharesModified implements StakingHooks.
func (h Hooks) BeforeDelegationSharesModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

// BeforeDelegationRemoved implements StakingHooks.
func (h Hooks) BeforeDelegationRemoved(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

// AfterDelegationModified implements StakingHooks.
func (h Hooks) AfterDelegationModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

// AfterUnbondingInitiated implements StakingHooks.
func (h Hooks) AfterUnbondingInitiated(_ context.Context, _ uint64) error { return nil }
