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

// slashPositionByUnbondingId subtracts slashAmount from a mapped position.
// No-op if unbondingId is not mapped to a tier position.
func (k Keeper) slashPositionByUnbondingId(ctx context.Context, unbondingId uint64, slashAmount math.Int) error {
	positionId, err := k.UnbondingDelegationMappings.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.getPosition(ctx, positionId)
	if errors.Is(err, types.ErrPositionNotFound) {
		// Stale mapping after position lifecycle completion.
		return k.deleteUnbondingPositionMapping(ctx, unbondingId)
	}
	if err != nil {
		return err
	}

	newAmount := math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt())
	pos.UpdateAmount(newAmount)

	return k.setPosition(ctx, pos)
}

// slashRedelegationPosition reduces both Amount and DelegatedShares for
// a position mapped to the given redelegation unbonding ID.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, slashAmount math.Int, shareBurnt math.LegacyDec) error {
	positionId, err := k.RedelegationMappings.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.getPosition(ctx, positionId)
	if errors.Is(err, types.ErrPositionNotFound) {
		return k.deleteRedelegationPositionMapping(ctx, unbondingId)
	}
	if err != nil {
		return err
	}

	newAmount := math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt())
	pos.UpdateAmount(newAmount)

	if pos.IsDelegated() && shareBurnt.IsPositive() {
		newShares := pos.DelegatedShares.Sub(shareBurnt)
		if newShares.IsPositive() {
			pos.UpdateDelegatedShares(newShares)
		} else {
			pos.ClearDelegation()
		}
	}

	return k.setPosition(ctx, pos)
}
