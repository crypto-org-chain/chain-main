package keeper

import (
	"context"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// calculateBonus returns accrued bonus, yielding zero when the validator is not bonded.
func (k Keeper) calculateBonus(position types.Position, validator stakingtypes.Validator, tier types.Tier, blockTime time.Time) math.Int {
	if !validator.IsBonded() {
		return math.ZeroInt()
	}
	return k.calculateBonusRaw(position, validator, tier, blockTime)
}

// calculateBonusRaw computes accrued bonus without checking validator status.
// Formula: tokens * BonusApy * durationSeconds / SecondsPerYear.
// accrualEnd is capped at ExitUnlockAt when the position is exiting.
func (k Keeper) calculateBonusRaw(position types.Position, validator stakingtypes.Validator, tier types.Tier, blockTime time.Time) math.Int {
	if !position.IsDelegated() {
		return math.ZeroInt()
	}

	if position.LastBonusAccrual.IsZero() {
		return math.ZeroInt()
	}

	accrualEnd := blockTime
	if position.CompletedExitLockDuration(blockTime) {
		accrualEnd = position.ExitUnlockAt
	}

	if !accrualEnd.After(position.LastBonusAccrual) {
		return math.ZeroInt()
	}

	durationSeconds := int64(accrualEnd.Sub(position.LastBonusAccrual) / time.Second)
	tokens := validator.TokensFromShares(position.DelegatedShares)

	return tokens.
		Mul(tier.BonusApy).
		MulInt64(durationSeconds).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()
}

func applyBonusAccrualCheckpoint(pos *types.Position, blockTime time.Time) {
	accrualEnd := blockTime
	if pos.CompletedExitLockDuration(blockTime) {
		accrualEnd = pos.ExitUnlockAt
	}
	pos.UpdateLastBonusAccrual(accrualEnd)
}

// bonusAccrualAmount returns bonus owed for pos at blockTime. When forceAccrue is true,
// bonded status is ignored (calculateBonusRaw).
func (k Keeper) bonusAccrualAmount(ctx context.Context, pos types.Position, val stakingtypes.Validator, tier types.Tier, blockTime time.Time, forceAccrue bool) (math.Int, error) {
	if forceAccrue {
		return k.calculateBonusRaw(pos, val, tier, blockTime), nil
	}

	if !pos.IsDelegated() || pos.LastBonusAccrual.IsZero() {
		return math.ZeroInt(), nil
	}

	accrualEnd := blockTime
	if pos.CompletedExitLockDuration(blockTime) {
		accrualEnd = pos.ExitUnlockAt
	}
	if !accrualEnd.After(pos.LastBonusAccrual) {
		return math.ZeroInt(), nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return math.ZeroInt(), err
	}

	tokens := val.TokensFromShares(pos.DelegatedShares)
	pauseAt, hasPause, err := k.getValidatorBonusPauseAt(ctx, valAddr)
	if err != nil {
		return math.ZeroInt(), err
	}
	if !hasPause {
		if !val.IsBonded() {
			return math.ZeroInt(), nil
		}
		durationSeconds := int64(accrualEnd.Sub(pos.LastBonusAccrual) / time.Second)
		return tokens.
			Mul(tier.BonusApy).
			MulInt64(durationSeconds).
			QuoInt64(types.SecondsPerYear).
			TruncateInt(), nil
	}

	resumeAt, hasResume, err := k.getValidatorBonusResumeAt(ctx, valAddr)
	if err != nil {
		return math.ZeroInt(), err
	}

	pauseRate, hasPauseRate, err := k.getValidatorBonusPauseRate(ctx, valAddr)
	if err != nil {
		return math.ZeroInt(), err
	}
	pauseTokens := tokens
	if hasPauseRate {
		pauseTokens = pos.DelegatedShares.Mul(pauseRate)
	}

	preDurationSeconds := int64(0)
	preEnd := minTime(accrualEnd, pauseAt)
	if preEnd.After(pos.LastBonusAccrual) {
		preDurationSeconds = int64(preEnd.Sub(pos.LastBonusAccrual) / time.Second)
	}

	postDurationSeconds := int64(0)
	if hasResume && !resumeAt.Before(pauseAt) {
		postStart := maxTime(pos.LastBonusAccrual, resumeAt)
		if accrualEnd.After(postStart) {
			postDurationSeconds = int64(accrualEnd.Sub(postStart) / time.Second)
		}
	}

	totalBonusDec := math.LegacyZeroDec()
	if preDurationSeconds > 0 {
		totalBonusDec = totalBonusDec.Add(
			pauseTokens.
				Mul(tier.BonusApy).
				MulInt64(preDurationSeconds).
				QuoInt64(types.SecondsPerYear),
		)
	}
	if postDurationSeconds > 0 {
		totalBonusDec = totalBonusDec.Add(
			tokens.
				Mul(tier.BonusApy).
				MulInt64(postDurationSeconds).
				QuoInt64(types.SecondsPerYear),
		)
	}
	return totalBonusDec.TruncateInt(), nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func (k Keeper) bonusCoinsIfPayable(ctx context.Context, pos types.Position, bonus math.Int) (sdk.Coins, error) {
	if bonus.IsZero() {
		return sdk.NewCoins(), nil
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return sdk.NewCoins(), err
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)
	if poolBalance.Amount.LT(bonus) {
		return sdk.NewCoins(), errorsmod.Wrapf(types.ErrInsufficientBonusPool, "bonus pool has insufficient funds, position id: %d, bonus: %s, pool balance: %s", pos.Id, bonus.String(), poolBalance.Amount.String())
	}

	return sdk.NewCoins(sdk.NewCoin(bondDenom, bonus)), nil
}

// sendBonusFromRewardsPool checks the rewards pool, transfers bonus to the owner, and emits EventBonusRewardsClaimed.
func (k Keeper) sendBonusFromRewardsPool(ctx context.Context, pos types.Position, bonus math.Int) (sdk.Coins, error) {
	bonusCoins, err := k.bonusCoinsIfPayable(ctx, pos, bonus)
	if err != nil {
		return sdk.NewCoins(), err
	}
	if bonusCoins.IsZero() {
		return sdk.NewCoins(), nil
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.NewCoins(), err
	}

	bonusCoin := bonusCoins[0]
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, bonusCoins); err != nil {
		return sdk.NewCoins(), err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBonusRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     bonusCoin,
	}); err != nil {
		return sdk.NewCoins(), err
	}

	return bonusCoins, nil
}
