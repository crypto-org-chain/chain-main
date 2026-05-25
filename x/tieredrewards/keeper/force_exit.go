// Can be deleted after v7.3.0 upgrade
package keeper

import (
	"context"
	"fmt"
	"math"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
)

func (k Keeper) ForceFullExitWithDelegation(ctx context.Context, posID uint64) error {
	posState, err := k.getPositionState(ctx, posID)
	if err != nil {
		return fmt.Errorf("get position %d: %w", posID, err)
	}
	if !posState.IsDelegated() {
		k.logger(ctx).Error("position is not delegated; cannot force full exit", "position_id", posID)
	}

	posState, _, _, err = k.claimRewards(ctx, posState)
	if err != nil {
		return fmt.Errorf("claim rewards for position %d: %w", posID, err)
	}

	valAddr, err := sdk.ValAddressFromBech32(posState.Delegation.ValidatorAddress)
	if err != nil {
		return fmt.Errorf("parse validator address for position %d: %w", posID, err)
	}

	positionAmount, err := k.reconcileAmountFromShares(ctx, valAddr, posState.Delegation.Shares)
	if err != nil {
		return fmt.Errorf("reconcile amount for position %d: %w", posID, err)
	}

	if _, _, _, err := k.transferDelegationFromPosition(ctx, posState, valAddr, positionAmount); err != nil {
		return fmt.Errorf("transfer delegation back to owner for position %d: %w", posID, err)
	}

	ownerAddr, err := sdk.AccAddressFromBech32(posState.Owner)
	if err != nil {
		return fmt.Errorf("parse owner address for position %d: %w", posID, err)
	}
	if err := k.alignVestingDelegationTracking(ctx, ownerAddr); err != nil {
		return fmt.Errorf("align vesting delegation tracking for position %d: %w", posID, err)
	}

	if err := k.deletePosition(ctx, posState.Position, &ValidatorTransition{PreviousAddress: valAddr.String()}); err != nil {
		return fmt.Errorf("delete position %d: %w", posID, err)
	}

	sdk.UnwrapSDKContext(ctx).Logger().Info("forced full tier exit",
		"position_id", posID,
		"owner", posState.Owner,
		"amount", positionAmount.String(),
		"validator", valAddr.String(),
	)
	return nil
}

// alignVestingDelegationTracking ensures that for a vesting account owner,
// DelegatedVesting + DelegatedFree matches the actual sum of on-chain
// delegations after a force-exit returns delegation back to the owner.
//
// transferDelegationFromPosition delegates to the owner with subtractAccount=false,
// which skips the bank-side TrackDelegation hook. For LockTier-origin positions
// this leaves DV+DF stale-low; for CommitDelegationToTier-origin positions DV+DF
// was already stale-high pre-migration and the returning delegation closes the
// gap. The diff-based top-up handles both, regardless of position origin or the
// order in which positions are exited.
func (k Keeper) alignVestingDelegationTracking(ctx context.Context, ownerAddr sdk.AccAddress) error {
	acc := k.accountKeeper.GetAccount(ctx, ownerAddr)
	if acc == nil {
		return nil
	}
	vacc, ok := acc.(sdkvesting.VestingAccount)
	if !ok {
		return nil
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return fmt.Errorf("bond denom: %w", err)
	}

	delegations, err := k.stakingKeeper.GetDelegatorDelegations(ctx, ownerAddr, math.MaxUint16)
	if err != nil {
		return fmt.Errorf("get delegator delegations: %w", err)
	}
	actualDelegated := sdkmath.ZeroInt()
	for _, d := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(d.GetValidatorAddr())
		if err != nil {
			return fmt.Errorf("parse validator address %q: %w", d.GetValidatorAddr(), err)
		}
		val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return fmt.Errorf("get validator %s: %w", valAddr, err)
		}
		actualDelegated = actualDelegated.Add(val.TokensFromShares(d.Shares).TruncateInt())
	}

	tracked := vacc.GetDelegatedVesting().AmountOf(bondDenom).Add(vacc.GetDelegatedFree().AmountOf(bondDenom))
	if !actualDelegated.GT(tracked) {
		return nil
	}
	deficit := actualDelegated.Sub(tracked)
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, deficit))
	// Pass balance == amount: TrackDelegation only uses balance for an
	// invariant check (amount <= balance); the DV/DF split is computed from
	// vestingCoins(blockTime) and DelegatedVesting. The owner's actual bank
	// balance is irrelevant here because the delegation came from the position
	// pool, not from the owner's balance.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	vacc.TrackDelegation(sdkCtx.BlockTime(), coins, coins)
	k.accountKeeper.SetAccount(ctx, vacc)
	return nil
}
