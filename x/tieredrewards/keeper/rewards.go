package keeper

import (
	"context"
	"errors"
	"time"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// Delegate delegates tokens from the tier module account to a validator on behalf of a position.
// Only bonded validators are allowed
// Returns the delegation shares received from the staking module.
func (k Keeper) Delegate(ctx context.Context, valAddr sdk.ValAddress, amount math.Int) (math.LegacyDec, error) {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, err
	}

	if !val.IsBonded() {
		return math.LegacyDec{}, types.ErrValidatorNotBonded
	}

	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)

	newShares, err := k.stakingKeeper.Delegate(ctx, moduleAddr, amount, stakingtypes.Unbonded, val, true)
	if err != nil {
		return math.LegacyDec{}, err
	}

	return newShares, nil
}

// withdrawDelegationRewards withdraws base staking rewards for the
// tier module account's delegation to a validator.
// Returns the rewards received.
func (k Keeper) withdrawDelegationRewards(ctx context.Context, valAddr sdk.ValAddress) (sdk.Coins, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	return k.distributionKeeper.WithdrawDelegationRewards(ctx, moduleAddr, valAddr)
}

// GetValidatorRewardRatio returns the cumulative rewards-per-share ratio for a validator.
// Returns empty DecCoins if no ratio has been stored yet (first delegation, no rewards accrued).
func (k Keeper) GetValidatorRewardRatio(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	ratio, err := k.ValidatorRewardRatio.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		// Not found — no rewards have been accrued yet for this validator.
		return sdk.DecCoins{}, nil
	}
	if err != nil {
		return sdk.DecCoins{}, err
	}
	return ratio.CumulativeRewardsPerShare, nil

}

// UpdateBaseRewardsPerShare withdraws base rewards from x/distribution for the
// tier module's delegation to the given validator and updates the cumulative
// rewards-per-share ratio stored for that validator.
//
// Must be called before any operation that changes the tier module's total
// delegation shares on a validator (new position, add to position, undelegate,
// redelegate) so that existing positions' share of prior rewards is preserved.
func (k Keeper) UpdateBaseRewardsPerShare(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	// Check if the tier module even has a delegation to this validator.
	poolAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	delegation, err := k.stakingKeeper.GetDelegation(ctx, poolAddr, valAddr)
	if errors.Is(err, stakingtypes.ErrNoDelegation) {
		// Not found — no rewards have been accrued yet for this validator.
		return sdk.DecCoins{}, nil
	}
	if err != nil {
		return sdk.DecCoins{}, err
	}

	totalShares := delegation.Shares
	if totalShares.IsZero() {
		// No delegation
		return sdk.DecCoins{}, nil
	}

	// Withdraw accumulated base rewards from distribution.
	rewards, err := k.withdrawDelegationRewards(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	if rewards.IsZero() {
		currentRatio, err := k.GetValidatorRewardRatio(ctx, valAddr)
		if errors.Is(err, collections.ErrNotFound) {
			// Not found — no rewards have been accrued yet for this validator.
			return sdk.DecCoins{}, nil
		}
		if err != nil {
			return sdk.DecCoins{}, err
		}
		return currentRatio, nil
	}

	decRewards := sdk.NewDecCoinsFromCoins(rewards...)
	delta := decRewards.QuoDecTruncate(totalShares)

	currentRatio, err := k.GetValidatorRewardRatio(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	newRatio := currentRatio.Add(delta...)

	err = k.ValidatorRewardRatio.Set(ctx, valAddr, types.ValidatorRewardRatio{
		CumulativeRewardsPerShare: newRatio,
	})

	if err != nil {
		return sdk.DecCoins{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsPerShareUpdated{
		Validator:                 valAddr.String(),
		RewardsWithdrawn:          rewards,
		CumulativeRewardsPerShare: newRatio,
	}); err != nil {
		return sdk.DecCoins{}, err
	}

	return newRatio, nil
}

// slashPositions slashes positions by a given fraction.
func (k Keeper) slashPositions(ctx context.Context, positions []types.Position, fraction sdkmath.LegacyDec) error {
	for _, pos := range positions {
		k.slashPosition(&pos, fraction)
		if err := k.SetPosition(ctx, pos); err != nil {
			return err
		}
	}
	return nil
}

// slashPosition reduces the amount of a position by a given fraction.
func (k Keeper) slashPosition(pos *types.Position, fraction sdkmath.LegacyDec) {
	slash := sdkmath.LegacyNewDecFromInt(pos.Amount).Mul(fraction).TruncateInt()
	pos.Amount = pos.Amount.Sub(slash)
	if pos.Amount.IsNegative() {
		pos.Amount = math.ZeroInt()
	}
}

// calculateBonus computes the accrued bonus for a position from LastRewardClaimedAt to accrualEnd.
// Formula: Amount × BonusApy × durationSeconds / SecondsPerYear
// accrualEnd is capped at ExitUnlockAt when the position is exiting.
func (k Keeper) calculateBonus(position types.Position, tier types.Tier, blockTime time.Time) math.Int {
	if position.LastBonusAccrual.IsZero() {
		return math.ZeroInt()
	}

	accrualEnd := blockTime
	if position.HasExited(blockTime) {
		accrualEnd = position.ExitUnlockAt
	}

	// No bonus if accrual end is not after last claimed
	if !accrualEnd.After(position.LastBonusAccrual) {
		return math.ZeroInt()
	}

	durationSeconds := int64(accrualEnd.Sub(position.LastBonusAccrual).Seconds())
	bonus := math.LegacyNewDecFromInt(position.Amount).
		Mul(tier.BonusApy).
		MulInt64(durationSeconds).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()

	return bonus
}

func (k Keeper) ClaimRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position) error {
	_, err := k.ClaimBaseRewardsForPositions(ctx, valAddr, positions)
	if err != nil {
		return err
	}

	_, err = k.ClaimBonusRewardsForPositions(ctx, positions)
	if err != nil && errors.Is(err, types.ErrInsufficientBonusPool) {
		k.Logger(ctx).Error("failed to claim bonus rewards due to insufficient funds in rewards pool before validator slashed",
			"validator", valAddr.String(),
			"error", err,
		)
		return nil
	}
	return err
}

// ClaimBaseRewardsForPositions updates the cumulative rewards-per-share ratio
// for the validator and then settles each position's base rewards using the
// difference between the current ratio and the position's starting snapshot.
func (k Keeper) ClaimBaseRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position) (sdk.Coins, error) {
	currentRatio, err := k.UpdateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	total := sdk.NewCoins()
	for i := range positions {
		claimed, err := k.ClaimBaseRewards(ctx, &positions[i], currentRatio)
		if err != nil {
			return sdk.Coins{}, err
		}
		total = total.Add(claimed...)

		if err := k.SetPosition(ctx, positions[i]); err != nil {
			return sdk.Coins{}, err
		}
	}

	return total, nil
}

// ClaimBaseRewards calculates and sends a position's accrued base rewards using
// the cumulative rewards-per-share ratio. Updates the position's snapshot to the
// current ratio so rewards are not double-counted.
//
// reward = position.DelegatedShares × (currentRatio − position.BaseRewardsPerShare)
func (k Keeper) ClaimBaseRewards(ctx context.Context, pos *types.Position, currentRatio sdk.DecCoins) (sdk.Coins, error) {
	if !pos.IsDelegated() || pos.DelegatedShares.IsZero() {
		return sdk.Coins{}, nil
	}

	// Compute the difference in cumulative ratio since the position was
	// created (or last claimed).
	delta := currentRatio.Sub(pos.BaseRewardsPerShare)
	if delta.IsZero() {
		// Update snapshot even if zero so future claims start from here.
		pos.BaseRewardsPerShare = currentRatio
		return sdk.Coins{}, nil
	}

	posRewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()

	// Update the position's snapshot to the current ratio.
	pos.BaseRewardsPerShare = currentRatio

	if posRewards.IsZero() {
		return sdk.Coins{}, nil
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.Coins{}, err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, posRewards); err != nil {
		return sdk.Coins{}, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     posRewards,
	}); err != nil {
		return sdk.Coins{}, err
	}

	return posRewards, nil
}

// ClaimBonusRewardsForPositions settles bonus for a list of positions.
// Caches tier lookups so positions in the same tier don't re-fetch.
func (k Keeper) ClaimBonusRewardsForPositions(ctx context.Context, positions []types.Position) (sdk.Coins, error) {
	tierCache := make(map[uint32]types.Tier)
	total := sdk.NewCoins()

	for _, pos := range positions {
		tier, ok := tierCache[pos.TierId]
		if !ok {
			tier, err := k.Tiers.Get(ctx, pos.TierId)
			if err != nil {
				return sdk.NewCoins(), err
			}
			tierCache[pos.TierId] = tier
		}

		bonus, err := k.ClaimBonusRewards(ctx, &pos, tier)
		if err != nil {
			return sdk.NewCoins(), err
		}

		total = total.Add(bonus...)

		if err := k.SetPosition(ctx, pos); err != nil {
			return sdk.NewCoins(), err
		}
	}

	return total, nil
}

// ClaimBonusRewards calculates and pays the bonus for a position from the rewards pool.
// Updates LastBonusAccrual on the position.
func (k Keeper) ClaimBonusRewards(ctx context.Context, pos *types.Position, tier types.Tier) (sdk.Coins, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	bonus := k.calculateBonus(*pos, tier, blockTime)

	accrualEnd := blockTime
	if pos.HasExited(blockTime) {
		accrualEnd = pos.ExitUnlockAt
	}
	pos.LastBonusAccrual = accrualEnd

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

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.NewCoins(), err
	}

	bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, bonus))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, bonusCoins); err != nil {
		return sdk.NewCoins(), err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBonusRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     sdk.NewCoin(bondDenom, bonus),
	}); err != nil {
		return sdk.NewCoins(), err
	}

	return bonusCoins, nil
}
