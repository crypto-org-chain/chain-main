package keeper

import (
	"context"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func applyBonusAccrualCheckpoint(pos *types.Position, blockTime time.Time) {
	accrualEnd := blockTime
	if pos.CompletedExitLockDuration(blockTime) {
		accrualEnd = pos.ExitUnlockAt
	}
	pos.UpdateLastBonusAccrual(accrualEnd)
}

// computeSegmentBonus computes bonus for a time segment using a snapshot rate.
// Formula: shares * tokensPerShare * tier.BonusApy * durationSeconds / SecondsPerYear
func (k Keeper) computeSegmentBonus(pos types.PositionState, tier types.Tier, segmentStart, segmentEnd time.Time, tokensPerShare math.LegacyDec) math.Int {
	if !pos.ExitUnlockAt.IsZero() && segmentEnd.After(pos.ExitUnlockAt) {
		segmentEnd = pos.ExitUnlockAt
	}

	if !segmentEnd.After(segmentStart) {
		return math.ZeroInt()
	}

	durationSeconds := int64(segmentEnd.Sub(segmentStart) / time.Second)
	if durationSeconds <= 0 {
		return math.ZeroInt()
	}

	tokens := pos.Delegation.Shares.Mul(tokensPerShare)

	return tokens.
		Mul(tier.BonusApy).
		MulInt64(durationSeconds).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()
}

func (k Keeper) sufficientBonusPoolBalance(ctx context.Context, bonus sdk.Coins) error {
	if bonus.IsZero() {
		return nil
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBalance := k.bankKeeper.GetAllBalances(ctx, poolAddr)
	if !poolBalance.IsAllGTE(bonus) {
		return errorsmod.Wrapf(types.ErrInsufficientBonusPool,
			"bonus: %s, pool balance: %s",
			bonus.String(), poolBalance.String())
	}

	return nil
}
