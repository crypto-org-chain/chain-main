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

// sendBonusFromRewardsPool checks the rewards pool, transfers bonus to the owner, and emits EventBonusRewardsClaimed.
func (k Keeper) sendBonusFromRewardsPool(ctx context.Context, pos types.Position, bonus math.Int) (sdk.Coins, error) {
	if bonus.IsZero() {
		return sdk.Coins{}, nil
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return sdk.Coins{}, err
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)

	if poolBalance.Amount.LT(bonus) {
		return sdk.Coins{}, errorsmod.Wrapf(types.ErrInsufficientBonusPool, "bonus pool has insufficient funds, position id: %d, bonus: %s, pool balance: %s", pos.Id, bonus.String(), poolBalance.Amount.String())
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.Coins{}, err
	}

	bonusCoin := sdk.NewCoin(bondDenom, bonus)
	bonusCoins := sdk.NewCoins(bonusCoin)
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, bonusCoins); err != nil {
		return sdk.Coins{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBonusRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     bonusCoin,
	}); err != nil {
		return sdk.Coins{}, err
	}

	return bonusCoins, nil
}

