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
func (k Keeper) bonusAccrualAmount(pos types.Position, val stakingtypes.Validator, tier types.Tier, blockTime time.Time, forceAccrue bool) math.Int {
	if forceAccrue {
		return k.calculateBonusRaw(pos, val, tier, blockTime)
	}
	return k.calculateBonus(pos, val, tier, blockTime)
}

func (k Keeper) checkBonusPoolBalance(ctx context.Context, bonus sdk.Coins) (sdk.Coins, error) {
	if bonus.IsZero() {
		return sdk.NewCoins(), nil
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetAllBalances(ctx, poolAddr)
	if !poolBalance.IsAllGTE(bonus) {
		return sdk.NewCoins(), errorsmod.Wrapf(types.ErrInsufficientBonusPool,
			"bonus pool has insufficient funds, bonus: %s, pool balance: %s",
			bonus.String(), poolBalance.String())
	}

	return bonus, nil
}
