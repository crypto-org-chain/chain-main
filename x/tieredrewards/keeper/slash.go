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

	pos.UpdateUndelegatedAmount(math.MaxInt(pos.UndelegatedAmount.Sub(slashAmount), math.ZeroInt()))

	return k.setPosition(ctx, pos)
}

// slashRedelegationPosition reduces DelegatedShares 
// for a position mapped to the given redelegation unbonding ID.
//
// Because the position is still delegated to the destination validator, we
// claim pending rewards BEFORE reducing shares. Otherwise, base rewards
// accrued at the pre-slash share count would be computed on fewer shares at
// claim time, losing the difference.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, slashAmount math.Int, shareBurnt math.LegacyDec) error {
	pos, found, err := k.getMappedSlashPosition(ctx, k.RedelegationMappings, unbondingId, k.deleteRedelegationPositionMapping)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	// Claim pending rewards before modifying shares so that base and bonus
	// rewards accumulated at the pre-slash share count are not lost.
	if pos.IsDelegated() {
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return err
		}

		currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
		if err != nil {
			return err
		}

		if _, err := k.claimBaseRewards(ctx, []*types.Position{&pos}, pos.Owner, valAddr, currentRatio); err != nil {
			return err
		}

		if _, err := k.processEventsAndClaimBonus(ctx, &pos, valAddr); err != nil {
			if errors.Is(err, types.ErrInsufficientBonusPool) {
				k.logger(ctx).Error("insufficient bonus pool during redelegation slash",
					"position_id", pos.Id,
					"error", err.Error(),
				)
			} else {
				return err
			}
		}
	}

	if pos.IsDelegated() && shareBurnt.IsPositive() {
		newShares := pos.DelegatedShares.Sub(shareBurnt)
		if newShares.IsPositive() {
			pos.UpdateDelegatedShares(newShares)
		} else {
			pos.ClearDelegation()
			// Defensive: ensures position amount is zero
			pos.UpdateUndelegatedAmount(math.ZeroInt())
		}
	}

	return k.setPosition(ctx, pos)
}
