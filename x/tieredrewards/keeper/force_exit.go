// Can be deleted after v7.3.0 upgrade
package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) ForceFullExitWithDelegation(ctx context.Context, posID uint64) error {
	posState, err := k.getPositionState(ctx, posID)
	if err != nil {
		return fmt.Errorf("get position %d: %w", posID, err)
	}
	if !posState.IsDelegated() {
		return fmt.Errorf("position %d is not delegated; cannot force full exit", posID)
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
