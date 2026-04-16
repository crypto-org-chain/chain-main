package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (k Keeper) slashPositions(ctx context.Context, val sdk.ValAddress, positions []types.Position, fraction math.LegacyDec) error {
	validator, err := k.stakingKeeper.GetValidator(ctx, val)
	if err != nil {
		return err
	}
	for i := range positions {
		k.slash(&positions[i], validator, fraction)
		if err := k.setPosition(ctx, positions[i]); err != nil {
			return err
		}
	}
	return nil
}

// slash updates a position's post-slash token amount from validator shares.
// LegacyDec rounding may differ from SDK accounting by up to 1 basecro.
// pos.Amount is reconciled with the SDK return value during TierUndelegate.
func (k Keeper) slash(pos *types.Position, validator stakingtypes.Validator, fraction math.LegacyDec) {
	postSlashTokens := validator.TokensFromShares(pos.DelegatedShares).Mul(math.LegacyOneDec().Sub(fraction)).TruncateInt()
	pos.UpdateAmount(math.MaxInt(postSlashTokens, math.ZeroInt()))
}

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

// slashRedelegationPosition reduces both Amount and DelegatedShares for
// a position mapped to the given redelegation unbonding ID.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, slashAmount math.Int, shareBurnt math.LegacyDec) error {
	pos, found, err := k.getMappedSlashPosition(ctx, k.RedelegationMappings, unbondingId, k.deleteRedelegationPositionMapping)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	pos.UpdateAmount(math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt()))

	if pos.IsDelegated() && shareBurnt.IsPositive() {
		newShares := pos.DelegatedShares.Sub(shareBurnt)
		if newShares.IsPositive() {
			pos.UpdateDelegatedShares(newShares)
		} else {
			pos.ClearDelegation()
			// ensures position amount is zero when all shares are burnt
			pos.UpdateAmount(math.ZeroInt())
		}
	}

	return k.setPosition(ctx, pos)
}
