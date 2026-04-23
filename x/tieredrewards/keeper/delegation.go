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

// delegate delegates tokens from the tier module account to a bonded validator.
func (k Keeper) delegate(ctx context.Context, valAddr sdk.ValAddress, amount math.Int) (math.LegacyDec, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if !val.IsBonded() {
		return math.LegacyDec{}, types.ErrValidatorNotBonded
	}

	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)

	return k.stakingKeeper.Delegate(ctx, moduleAddr, amount, stakingtypes.Unbonded, val, true)
}

func (k Keeper) undelegate(ctx context.Context, valAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.Int, uint64, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	completionTime, returnAmount, unbondingId, err := k.stakingKeeper.Undelegate(ctx, moduleAddr, valAddr, shares)
	if err != nil {
		return time.Time{}, math.Int{}, 0, err
	}
	return completionTime, returnAmount, unbondingId, nil
}

// redelegate moves a delegation between validators for the tier module account.
// The caller must store the returned unbondingId for slash tracking.
func (k Keeper) redelegate(ctx context.Context, srcValAddr, dstValAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.LegacyDec, uint64, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	completionTime, newShares, unbondingId, err := k.stakingKeeper.BeginRedelegation(ctx, moduleAddr, srcValAddr, dstValAddr, shares)
	if err != nil {
		return time.Time{}, math.LegacyDec{}, 0, err
	}
	return completionTime, newShares, unbondingId, nil
}
