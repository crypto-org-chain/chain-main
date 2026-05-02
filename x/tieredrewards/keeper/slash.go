package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) getMappedSlashPosition(
	ctx context.Context,
	mappings *collections.IndexedMap[uint64, uint64, UnbondingMappingsIndexes],
	unbondingId uint64,
	deleteMapping func(context.Context, uint64) error,
) (types.Position, bool, error) {
	positionId, err := mappings.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return types.Position{}, false, nil
	}
	if err != nil {
		return types.Position{}, false, err
	}

	pos, err := k.getPosition(ctx, positionId)
	if errors.Is(err, types.ErrPositionNotFound) {
		// Stale mapping after position lifecycle completion.
		return types.Position{}, false, deleteMapping(ctx, unbondingId)
	}
	if err != nil {
		return types.Position{}, false, err
	}

	return pos, true, nil
}

// slashPositionByUnbondingId subtracts slashAmount from a mapped position.
// No-op if unbondingId is not mapped to a tier position.
func (k Keeper) slashPositionByUnbondingId(ctx context.Context, unbondingId uint64, slashAmount math.Int) error {
	pos, found, err := k.getMappedSlashPosition(ctx, k.UnbondingDelegationMappings, unbondingId, k.deleteUnbondingPositionMapping)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	pos.UpdateAmount(math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt()))

	return k.setPosition(ctx, pos)
}

// slashRedelegationPosition reduces DelegatedShares for a position mapped to
// the given redelegation unbonding ID.
//
// Base rewards will already have been auto-withdrawn to the owner BeforeDelegationSharesModified hook in
// distribution module's Unbond path during slash by the time this hook fires.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, shareBurnt math.LegacyDec) error {
	pos, found, err := k.getMappedSlashPosition(ctx, k.RedelegationMappings, unbondingId, k.deleteRedelegationPositionMapping)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	if !pos.IsDelegated() {
		return nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return err
	}

	if _, err := k.processEventsAndClaimBonus(ctx, &pos, valAddr); err != nil {
		// deliberate forgo of bonus rewards even if pool is insufficient toprevent chain halt
		if errors.Is(err, types.ErrInsufficientBonusPool) {
			k.logger(ctx).Error("insufficient bonus pool during redelegation slash",
				"position_id", pos.Id,
				"error", err.Error(),
			)
		} else {
			return err
		}
	}

	if shareBurnt.IsPositive() {
		newShares := pos.DelegatedShares.Sub(shareBurnt)
		if newShares.IsPositive() {
			pos.UpdateDelegatedShares(newShares)
		} else {
			pos.ClearDelegation()
			// Defensive: ensures position amount is zero
			pos.UpdateAmount(math.ZeroInt())
		}
	}

	return k.setPosition(ctx, pos)
}
