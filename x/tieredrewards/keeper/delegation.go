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

func (k Keeper) undelegate(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.Int, error) {
	return k.stakingKeeper.Undelegate(ctx, delAddr, valAddr, shares)
}

func (k Keeper) redelegate(ctx context.Context, delAddr sdk.AccAddress, srcValAddr, dstValAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, uint64, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, dstValAddr)
	if err != nil {
		return time.Time{}, 0, err
	}

	if !val.IsBonded() {
		return time.Time{}, 0, types.ErrValidatorNotBonded
	}

	return k.stakingKeeper.BeginRedelegation(ctx, delAddr, srcValAddr, dstValAddr, shares)
}

func (k Keeper) getDelegation(ctx context.Context, positionId uint64) (*stakingtypes.Delegation, error) {
	// A position has at most one delegation.
	dels, err := k.stakingKeeper.GetDelegatorDelegations(ctx, types.GetDelegatorAddress(positionId), 1)
	if err != nil {
		return nil, err
	}
	if len(dels) == 0 {
		return nil, nil
	}
	d := dels[0]
	return &d, nil
}

func (k Keeper) isRedelegating(ctx context.Context, positionId uint64) (bool, error) {
	reds, err := k.stakingKeeper.GetRedelegations(ctx, types.GetDelegatorAddress(positionId), 1)
	if err != nil {
		return false, err
	}
	return len(reds) > 0, nil
}

func (k Keeper) isUnbonding(ctx context.Context, positionId uint64) (bool, error) {
	ubds, err := k.stakingKeeper.GetUnbondingDelegations(ctx, types.GetDelegatorAddress(positionId), 1)
	if err != nil {
		return false, err
	}
	return len(ubds) > 0, nil
}
