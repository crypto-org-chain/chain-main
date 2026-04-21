package keeper

import (
	"context"
	stderrors "errors"
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) setValidatorBonusPauseAt(ctx context.Context, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.setValidatorBonusPauseAtUnix(ctx, valAddr, sdkCtx.BlockTime().Unix())
}

func (k Keeper) setValidatorBonusPauseAtUnix(ctx context.Context, valAddr sdk.ValAddress, unixTs int64) error {
	return k.ValidatorBonusPauseAt.Set(ctx, valAddr, unixTs)
}

func (k Keeper) getValidatorBonusPauseAt(ctx context.Context, valAddr sdk.ValAddress) (time.Time, bool, error) {
	unixTs, err := k.ValidatorBonusPauseAt.Get(ctx, valAddr)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return time.Unix(unixTs, 0), true, nil
}

func (k Keeper) clearValidatorBonusPauseAt(ctx context.Context, valAddr sdk.ValAddress) error {
	err := k.ValidatorBonusPauseAt.Remove(ctx, valAddr)
	if err != nil && !stderrors.Is(err, collections.ErrNotFound) {
		return err
	}
	return nil
}

func (k Keeper) setValidatorBonusResumeAt(ctx context.Context, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.setValidatorBonusResumeAtUnix(ctx, valAddr, sdkCtx.BlockTime().Unix())
}

func (k Keeper) setValidatorBonusResumeAtUnix(ctx context.Context, valAddr sdk.ValAddress, unixTs int64) error {
	return k.ValidatorBonusResumeAt.Set(ctx, valAddr, unixTs)
}

func (k Keeper) getValidatorBonusResumeAt(ctx context.Context, valAddr sdk.ValAddress) (time.Time, bool, error) {
	unixTs, err := k.ValidatorBonusResumeAt.Get(ctx, valAddr)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return time.Unix(unixTs, 0), true, nil
}

func (k Keeper) clearValidatorBonusResumeAt(ctx context.Context, valAddr sdk.ValAddress) error {
	err := k.ValidatorBonusResumeAt.Remove(ctx, valAddr)
	if err != nil && !stderrors.Is(err, collections.ErrNotFound) {
		return err
	}
	return nil
}

func (k Keeper) setValidatorBonusPauseRate(ctx context.Context, valAddr sdk.ValAddress, tokensPerShare sdkmath.LegacyDec) error {
	return k.ValidatorBonusPauseRate.Set(ctx, valAddr, tokensPerShare)
}

func (k Keeper) getValidatorBonusPauseRate(ctx context.Context, valAddr sdk.ValAddress) (sdkmath.LegacyDec, bool, error) {
	rate, err := k.ValidatorBonusPauseRate.Get(ctx, valAddr)
	if err != nil {
		if stderrors.Is(err, collections.ErrNotFound) {
			return sdkmath.LegacyZeroDec(), false, nil
		}
		return sdkmath.LegacyZeroDec(), false, err
	}
	return rate, true, nil
}

func (k Keeper) clearValidatorBonusPauseRate(ctx context.Context, valAddr sdk.ValAddress) error {
	err := k.ValidatorBonusPauseRate.Remove(ctx, valAddr)
	if err != nil && !stderrors.Is(err, collections.ErrNotFound) {
		return err
	}
	return nil
}
