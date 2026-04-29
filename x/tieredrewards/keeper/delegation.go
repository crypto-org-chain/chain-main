package keeper

import (
	"context"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// getTokensPerShare returns the validator's current token-per-share rate.
func (k Keeper) getTokensPerShare(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}
	if val.GetDelegatorShares().IsZero() {
		return math.LegacyZeroDec(), nil
	}
	return val.TokensFromShares(math.LegacyOneDec()), nil
}

// positionTokenValue returns the live token value of a position:
// TokensFromShares for delegated, Amount for undelegated.
func (k Keeper) positionTokenValue(ctx context.Context, pos types.Position) (math.Int, error) {
	if !pos.IsDelegated() {
		return pos.Amount, nil
	}
	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return math.Int{}, err
	}
	return k.reconcileAmountFromShares(ctx, valAddr, pos.DelegatedShares)
}

// reconcileAmountFromShares converts delegation shares to the actual withdrawable
// token amount under the validator's current exchange rate.
func (k Keeper) reconcileAmountFromShares(ctx context.Context, valAddr sdk.ValAddress, shares math.LegacyDec) (math.Int, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.Int{}, err
	}
	if val.GetDelegatorShares().IsZero() {
		return math.ZeroInt(), nil
	}
	return val.TokensFromShares(shares).TruncateInt(), nil
}

// updateDelegation updates a position's delegation fields.
// For delegated positions, Amount is always zero.
func (k Keeper) updateDelegation(ctx context.Context, pos *types.Position, delegation types.Delegation) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pos.WithDelegation(delegation, sdkCtx.BlockTime())
	return nil
}

func (k Keeper) delegate(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, amount math.Int) (math.LegacyDec, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if !val.IsBonded() {
		return math.LegacyDec{}, types.ErrValidatorNotBonded
	}

	return k.stakingKeeper.Delegate(ctx, delAddr, amount, stakingtypes.Unbonded, val, true)
}

func (k Keeper) undelegate(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.Int, uint64, error) {
	return k.stakingKeeper.Undelegate(ctx, delAddr, valAddr, shares)
}

func (k Keeper) redelegate(ctx context.Context, delAddr sdk.AccAddress, srcValAddr, dstValAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.LegacyDec, uint64, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, dstValAddr)
	if err != nil {
		return time.Time{}, math.LegacyDec{}, 0, err
	}

	if !val.IsBonded() {
		return time.Time{}, math.LegacyDec{}, 0, types.ErrValidatorNotBonded
	}

	return k.stakingKeeper.BeginRedelegation(ctx, delAddr, srcValAddr, dstValAddr, shares)
}
