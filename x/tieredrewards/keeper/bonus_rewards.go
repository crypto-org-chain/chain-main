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

type validatorBonusAccrualState struct {
	pauseAt      time.Time
	hasPause     bool
	resumeAt     time.Time
	hasResume    bool
	pauseRate    math.LegacyDec
	hasPauseRate bool
}

func (k Keeper) loadValidatorBonusAccrualState(ctx context.Context, valAddr sdk.ValAddress) (validatorBonusAccrualState, error) {
	pauseAt, hasPause, err := k.getValidatorBonusPauseAt(ctx, valAddr)
	if err != nil {
		return validatorBonusAccrualState{}, err
	}
	resumeAt, hasResume, err := k.getValidatorBonusResumeAt(ctx, valAddr)
	if err != nil {
		return validatorBonusAccrualState{}, err
	}
	pauseRate, hasPauseRate, err := k.getValidatorBonusPauseRate(ctx, valAddr)
	if err != nil {
		return validatorBonusAccrualState{}, err
	}
	return validatorBonusAccrualState{
		pauseAt:      pauseAt,
		hasPause:     hasPause,
		resumeAt:     resumeAt,
		hasResume:    hasResume,
		pauseRate:    pauseRate,
		hasPauseRate: hasPauseRate,
	}, nil
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
// bonded-status gating is ignored but validator pause/resume checkpoints are still enforced.
func (k Keeper) bonusAccrualAmount(pos types.Position, val stakingtypes.Validator, tier types.Tier, blockTime time.Time, forceAccrue bool, bonusState validatorBonusAccrualState) (math.Int, error) {
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

	tokens := val.TokensFromShares(pos.DelegatedShares)
	if !bonusState.hasPause {
		if !forceAccrue && !val.IsBonded() {
			return math.ZeroInt(), nil
		}
		durationSeconds := int64(accrualEnd.Sub(pos.LastBonusAccrual) / time.Second)
		return tokens.
			Mul(tier.BonusApy).
			MulInt64(durationSeconds).
			QuoInt64(types.SecondsPerYear).
			TruncateInt(), nil
	}

	pauseTokens := tokens
	if bonusState.hasPauseRate {
		pauseTokens = pos.DelegatedShares.Mul(bonusState.pauseRate)
	}

	preDurationSeconds := int64(0)
	preEnd := minTime(accrualEnd, bonusState.pauseAt)
	if preEnd.After(pos.LastBonusAccrual) {
		preDurationSeconds = int64(preEnd.Sub(pos.LastBonusAccrual) / time.Second)
	}

	postDurationSeconds := int64(0)
	if bonusState.hasResume && !bonusState.resumeAt.Before(bonusState.pauseAt) {
		postStart := maxTime(pos.LastBonusAccrual, bonusState.resumeAt)
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
