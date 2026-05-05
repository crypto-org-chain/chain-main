package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// slashRedelegationPosition processes bonus rewards owed up to the slash point
// for a redelegating position, and reconciles the validator-position counter if
// the slash fully burned the destination delegation.
//
// Base rewards will already have been auto-withdrawn to the owner via
// distribution's BeforeDelegationSharesModified hook by the time this fires.
func (k Keeper) slashRedelegationPosition(ctx context.Context, delAddr sdk.AccAddress, dstValAddr sdk.ValAddress) error {
	positionId, err := k.getRedelegatingPositionByAddr(ctx, delAddr)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.loadPositionState(ctx, positionId)
	if errors.Is(err, types.ErrPositionNotFound) {
		k.logger(ctx).Error("position not found after redelegation slash",
			"position_id", positionId,
			"error", err.Error(),
		)
		// Defensive: clean the stale mapping entry.
		return k.deleteRedelegatingPosition(ctx, delAddr)
	}
	if err != nil {
		return err
	}

	if _, err := k.processEventsAndClaimBonus(ctx, &pos); err != nil {
		// Deliberately forgo bonus rewards if pool is insufficient to prevent chain halt.
		if errors.Is(err, types.ErrInsufficientBonusPool) {
			k.logger(ctx).Error("insufficient bonus pool during redelegation slash",
				"position_id", pos.Id,
				"error", err.Error(),
			)
		} else {
			return err
		}
	}

	if !pos.IsDelegated() {
		pos.ResetBonusCheckpoints()
		return k.setPosition(ctx, pos.Position, &ValidatorUpdate{Previous: dstValAddr.String()})
	}

	// No validator change in position is still delegated
	return k.setPosition(ctx, pos.Position, nil)
}
