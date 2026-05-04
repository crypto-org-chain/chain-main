package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
)

// slashRedelegationPosition processes bonus rewards owed up to the slash point
// for a redelegating positions.
//
// Base rewards will already have been auto-withdrawn to the owner via
// distribution's BeforeDelegationSharesModified hook by the time this fires.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, _ math.LegacyDec) error {
	positionId, err := k.RedelegationMappings.Get(ctx, unbondingId)
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
		return k.deleteRedelegationPositionMapping(ctx, unbondingId)
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
	}

	// Partial slash keeps the same validator → no attribution change, pass
	// nil. Full slash (pos.Delegation == nil) removes the delegation, but we
	// don't have the pre-slash validator here, so the counter stays stale
	// until `AfterRedelegationSlashed` carries the dst validator.
	return k.setPosition(ctx, pos.Position, nil)
}
