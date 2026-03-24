package keeper

import (
	"context"
	"errors"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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

	return k.stakingKeeper.Delegate(ctx, moduleAddr, amount, stakingtypes.Unbonded, val, true)
}

// Undelegate undelegates tokens from a validator on behalf of the tier module account.
// Returns the completion time and the unbonding ID for slash tracking.
func (k Keeper) Undelegate(ctx context.Context, valAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, uint64, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	completionTime, _, unbondingId, err := k.stakingKeeper.Undelegate(ctx, moduleAddr, valAddr, shares)
	if err != nil {
		return time.Time{}, 0, err
	}
	return completionTime, unbondingId, nil
}

// Redelegate redelegates tokens from one validator to another on behalf of the tier module account.
// Returns the completion time, new shares on the destination validator, and the unbonding ID for slash tracking.
func (k Keeper) Redelegate(ctx context.Context, srcValAddr, dstValAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.LegacyDec, uint64, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	completionTime, newShares, unbondingId, err := k.stakingKeeper.BeginRedelegation(ctx, moduleAddr, srcValAddr, dstValAddr, shares)
	if err != nil {
		return time.Time{}, math.LegacyDec{}, 0, err
	}
	return completionTime, newShares, unbondingId, nil
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
	// GetValidatorRewardRatio already normalises ErrNotFound to (sdk.DecCoins{}, nil).
	currentRatio, err := k.GetValidatorRewardRatio(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	// Check if the tier module even has a delegation to this validator.
	poolAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	delegation, err := k.stakingKeeper.GetDelegation(ctx, poolAddr, valAddr)
	if errors.Is(err, stakingtypes.ErrNoDelegation) {
		return sdk.DecCoins{}, nil
	}

	if err != nil {
		return sdk.DecCoins{}, err
	}

	totalShares := delegation.Shares
	// defensive, though it should never happen
	if totalShares.IsZero() {
		return sdk.DecCoins{}, nil
	}

	// Withdraw accumulated base rewards from distribution.
	rewards, err := k.withdrawDelegationRewards(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	if rewards.IsZero() {
		return currentRatio, nil
	}

	decRewards := sdk.NewDecCoinsFromCoins(rewards...)
	delta := decRewards.QuoDecTruncate(totalShares)

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
func (k Keeper) slashPositions(ctx context.Context, val sdk.ValAddress, positions []types.Position, fraction math.LegacyDec) error {
	validator, err := k.stakingKeeper.GetValidator(ctx, val)
	if err != nil {
		return err
	}
	for i := range positions {
		k.slash(&positions[i], validator, fraction)
		if err := k.SetPosition(ctx, positions[i]); err != nil {
			return err
		}
	}
	return nil
}

// slash reduces the bonded tokens of a position by a given fraction.
func (k Keeper) slash(pos *types.Position, validator stakingtypes.Validator, fraction math.LegacyDec) {
	bondedTokens := validator.TokensFromShares(pos.DelegatedShares)

	slash := bondedTokens.Mul(fraction).TruncateInt()
	amount := math.MaxInt(pos.Amount.Sub(slash), math.ZeroInt())
	pos.UpdateAmount(amount)
}

// slashPositionByUnbondingId looks up the position associated with an unbonding
// or redelegation entry and reduces its Amount by the given slashed amount.
// Called by AfterSlashUnbondingDelegation, AfterSlashUnbondingRedelegation, and
// AfterSlashRedelegation hooks.
// If the unbondingId is not mapped to a tier position (i.e. it belongs to a
// non-tier delegator), this is a no-op.
func (k Keeper) slashPositionByUnbondingId(ctx context.Context, unbondingId uint64, slashAmount math.Int) error {
	positionId, err := k.UnbondingIdToPositionId.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		// Not a tier module unbonding — ignore.
		return nil
	} else if err != nil {
		return err
	}

	pos, err := k.GetPosition(ctx, positionId)
	if err != nil {
		return err
	}

	newAmount := math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt())

	pos.UpdateAmount(newAmount)

	return k.SetPosition(ctx, pos)
}

// calculateBonus computes the accrued bonus for a position from LastRewardClaimedAt to accrualEnd.
// Formula: tokens × BonusApy × durationSeconds / SecondsPerYear
// where tokens = validator.TokensFromShares(DelegatedShares) — this uses the current exchange
// rate and reflects any slashing that has already been applied to the validator. As a result,
// after a 100% slash the position's DelegatedShares are worthless (TokensFromShares ≈ 0) and
// no further bonus accrues, even though Amount may still show a pre-slash value. This is the
// intended behavior: the position has no economic value remaining.
// accrualEnd is capped at ExitUnlockAt when the position is exiting.
// Returns both the bonus amount and the accrual end time so callers do not recompute it.
func (k Keeper) calculateBonus(position types.Position, validator stakingtypes.Validator, tier types.Tier, blockTime time.Time) (math.Int, time.Time) {
	if position.LastBonusAccrual.IsZero() {
		return math.ZeroInt(), blockTime
	}

	accrualEnd := blockTime
	if position.CompletedExitLockDuration(blockTime) {
		accrualEnd = position.ExitUnlockAt
	}

	// No bonus if accrual end is not after last claimed
	if !accrualEnd.After(position.LastBonusAccrual) {
		return math.ZeroInt(), accrualEnd
	}

	// Use integer division to avoid float64 truncation bias.
	durationSeconds := int64(accrualEnd.Sub(position.LastBonusAccrual) / time.Second)
	tokens := validator.TokensFromShares(position.DelegatedShares)

	bonus := tokens.
		Mul(tier.BonusApy).
		MulInt64(durationSeconds).
		QuoInt64(types.SecondsPerYear).
		TruncateInt()

	return bonus, accrualEnd
}

func (k Keeper) ClaimRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position) (sdk.Coins, sdk.Coins, error) {
	baseRewards, err := k.ClaimBaseRewardsForPositions(ctx, valAddr, positions)
	if err != nil {
		return sdk.Coins{}, sdk.Coins{}, err
	}
	bonusRewards, err := k.ClaimBonusRewardsForPositions(ctx, positions)
	if errors.Is(err, types.ErrInsufficientBonusPool) {
		// Bonus pool exhausted — base rewards were still claimed successfully.
		// Return base rewards with empty bonus; the un-paid bonus remains
		// accrued on the position and will be claimable once the pool is refilled.
		//
		// Partial-failure invariant: ClaimBonusRewardsForPositions stops at the
		// first position that cannot be paid. Positions before that one have had
		// their LastBonusAccrual advanced (and are persisted); the failing position
		// and all subsequent ones retain their previous LastBonusAccrual and will
		// recalculate their full unpaid bonus on the next successful claim.
		return baseRewards, sdk.Coins{}, nil
	}
	if err != nil {
		return sdk.Coins{}, sdk.Coins{}, err
	}
	return baseRewards, bonusRewards, nil
}

// ClaimAndRefreshPosition claims rewards for a single position then re-fetches it
// from the store. ClaimRewardsForPositions writes updated state via SetPosition, so
// any in-memory copy of the position is stale after the call. Callers that need to
// continue mutating the position (delegate, redelegate, add tokens) must use the
// returned refreshed copy.
func (k Keeper) ClaimAndRefreshPosition(ctx context.Context, valAddr sdk.ValAddress, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	base, bonus, err := k.ClaimRewardsForPositions(ctx, valAddr, []types.Position{pos})
	if err != nil {
		return types.Position{}, nil, nil, err
	}
	refreshed, err := k.GetPosition(ctx, pos.Id)
	if err != nil {
		return types.Position{}, nil, nil, err
	}
	return refreshed, base, bonus, nil
}

// ClaimBaseRewardsForPositions updates the cumulative rewards-per-share ratio
// for the validator and then settles each position's base rewards using the
// difference between the current ratio and the position's starting snapshot.
func (k Keeper) ClaimBaseRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position) (sdk.Coins, error) {
	currentRatio, err := k.UpdateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	total := sdk.Coins{}
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
	if !pos.IsDelegated() {
		return sdk.Coins{}, nil
	}

	// Compute the difference in cumulative ratio since the position was
	// created (or last claimed).
	delta := currentRatio.Sub(pos.BaseRewardsPerShare)

	if delta.IsAnyNegative() {
		k.Logger(ctx).Error("base rewards per share is negative, this should not happen, skipping base rewards claim", "position", pos.String())
		return sdk.Coins{}, nil
	}

	if delta.IsZero() {
		// Ensure that base rewards per share is updated to the current ratio
		pos.UpdateBaseRewardsPerShare(currentRatio)
		return sdk.Coins{}, nil
	}

	posRewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()

	// Update the position's snapshot to the current ratio.
	pos.UpdateBaseRewardsPerShare(currentRatio)

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
// Uses pointer access (&positions[i]) so callers' slice elements are updated in-place,
// consistent with ClaimBaseRewardsForPositions.
//
// Partial-failure behavior: if a position cannot be paid because the bonus pool is
// exhausted (ErrInsufficientBonusPool), the function returns that error immediately.
// Positions processed before the failing one have their LastBonusAccrual advanced and
// are already persisted; the failing position and all later ones are left untouched so
// their full unpaid bonus can be re-computed on a future claim.
func (k Keeper) ClaimBonusRewardsForPositions(ctx context.Context, positions []types.Position) (sdk.Coins, error) {
	tierCache := make(map[uint32]types.Tier)
	total := sdk.NewCoins()

	for i := range positions {
		tier, ok := tierCache[positions[i].TierId]
		if !ok {
			var err error
			tier, err = k.GetTier(ctx, positions[i].TierId)
			if err != nil {
				return sdk.Coins{}, err
			}
			tierCache[positions[i].TierId] = tier
		}

		bonus, err := k.ClaimBonusRewards(ctx, &positions[i], tier)
		if err != nil {
			return sdk.Coins{}, err
		}

		total = total.Add(bonus...)

		if err := k.SetPosition(ctx, positions[i]); err != nil {
			return sdk.Coins{}, err
		}
	}

	return total, nil
}

// ClaimBonusRewards calculates and pays the bonus for a position from the rewards pool.
// Updates LastBonusAccrual on the position.
//
// Ordering note: LastBonusAccrual is advanced to accrualEnd before the pool balance
// check. If the pool is insufficient the function returns ErrInsufficientBonusPool
// without persisting the position — the in-memory update is intentionally discarded
// by ClaimBonusRewardsForPositions, which only calls SetPosition on success. This
// means the position retains its previous LastBonusAccrual in the store and the full
// unpaid bonus will be recalculated on the next successful claim.
func (k Keeper) ClaimBonusRewards(ctx context.Context, pos *types.Position, tier types.Tier) (sdk.Coins, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return sdk.Coins{}, err
	}

	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	bonus, accrualEnd := k.calculateBonus(*pos, val, tier, blockTime)

	pos.UpdateLastBonusAccrual(accrualEnd)

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

	bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, bonus))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, bonusCoins); err != nil {
		return sdk.Coins{}, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBonusRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     sdk.NewCoin(bondDenom, bonus),
	}); err != nil {
		return sdk.Coins{}, err
	}

	return bonusCoins, nil
}
