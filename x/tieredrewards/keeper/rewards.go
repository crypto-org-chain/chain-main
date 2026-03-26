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

func (k Keeper) undelegate(ctx context.Context, valAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, uint64, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	completionTime, _, unbondingId, err := k.stakingKeeper.Undelegate(ctx, moduleAddr, valAddr, shares)
	if err != nil {
		return time.Time{}, 0, err
	}
	return completionTime, unbondingId, nil
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

func (k Keeper) withdrawDelegationRewards(ctx context.Context, valAddr sdk.ValAddress) (sdk.Coins, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	return k.distributionKeeper.WithdrawDelegationRewards(ctx, moduleAddr, valAddr)
}

func (k Keeper) getValidatorRewardRatio(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	ratio, err := k.ValidatorRewardRatio.Get(ctx, valAddr)
	if errors.Is(err, collections.ErrNotFound) {
		return sdk.DecCoins{}, nil
	}
	if err != nil {
		return sdk.DecCoins{}, err
	}
	return ratio.CumulativeRewardsPerShare, nil
}

// updateBaseRewardsPerShare withdraws base rewards from x/distribution and
// updates the cumulative rewards-per-share ratio for the given validator.
// Must be called before any operation that changes the module's delegation shares.
func (k Keeper) updateBaseRewardsPerShare(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	currentRatio, err := k.getValidatorRewardRatio(ctx, valAddr)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	delegation, err := k.stakingKeeper.GetDelegation(ctx, poolAddr, valAddr)
	if errors.Is(err, stakingtypes.ErrNoDelegation) {
		return sdk.DecCoins{}, nil
	}
	if err != nil {
		return sdk.DecCoins{}, err
	}

	totalShares := delegation.Shares
	if totalShares.IsZero() {
		return sdk.DecCoins{}, nil
	}

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

func (k Keeper) slashPositions(ctx context.Context, val sdk.ValAddress, positions []types.Position, fraction math.LegacyDec) error {
	validator, err := k.stakingKeeper.GetValidator(ctx, val)
	if err != nil {
		return err
	}
	for i := range positions {
		k.slash(&positions[i], validator, fraction)
		if err := k.setPosition(ctx, positions[i]); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) slash(pos *types.Position, validator stakingtypes.Validator, fraction math.LegacyDec) {
	bondedTokens := validator.TokensFromShares(pos.DelegatedShares)

	slash := bondedTokens.Mul(fraction).TruncateInt()
	amount := math.MaxInt(pos.Amount.Sub(slash), math.ZeroInt())
	pos.UpdateAmount(amount)
}

// slashPositionByUnbondingId reduces a position's Amount by the slashed amount.
// No-op if the unbondingId is not mapped to a tier position.
func (k Keeper) slashPositionByUnbondingId(ctx context.Context, unbondingId uint64, slashAmount math.Int) error {
	positionId, err := k.UnbondingIdToPositionId.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.getPosition(ctx, positionId)
	if err != nil {
		return err
	}

	newAmount := math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt())
	pos.UpdateAmount(newAmount)

	return k.setPosition(ctx, pos)
}

// slashRedelegationPosition reduces both Amount and DelegatedShares for
// a position mapped to the given unbonding ID.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, slashAmount math.Int, shareBurnt math.LegacyDec) error {
	positionId, err := k.UnbondingIdToPositionId.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.Positions.Get(ctx, positionId)
	if err != nil {
		return err
	}

	pos.UpdateAmount(math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt()))

	if pos.IsDelegated() && shareBurnt.IsPositive() {
		newShares := pos.DelegatedShares.Sub(shareBurnt)
		if newShares.IsPositive() {
			pos.UpdateDelegatedShares(newShares)
		} else {
			pos.ClearDelegation()
		}
	}

	return k.setPosition(ctx, pos)
}

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

// claimRewardsForPositions settles base and bonus rewards for a set of positions.
// When forceAccrue is true, bonus is calculated regardless of validator bonded status.
func (k Keeper) claimRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position, forceAccrue bool) (sdk.Coins, sdk.Coins, error) {
	baseRewards, err := k.claimBaseRewardsForPositions(ctx, valAddr, positions)
	if err != nil {
		return sdk.Coins{}, sdk.Coins{}, err
	}
	bonusRewards, err := k.claimBonusRewardsForPositions(ctx, positions, forceAccrue)
	if err != nil {
		return sdk.Coins{}, sdk.Coins{}, err
	}
	return baseRewards, bonusRewards, nil
}

// claimAndRefreshPosition claims rewards for a single position then re-fetches
// it from the store, since claimRewardsForPositions persists updates internally.
func (k Keeper) claimAndRefreshPosition(ctx context.Context, valAddr sdk.ValAddress, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	base, bonus, err := k.claimRewardsForPositions(ctx, valAddr, []types.Position{pos}, false)
	if err != nil {
		return types.Position{}, nil, nil, err
	}
	refreshed, err := k.getPosition(ctx, pos.Id)
	if err != nil {
		return types.Position{}, nil, nil, err
	}
	return refreshed, base, bonus, nil
}

func (k Keeper) claimBaseRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position) (sdk.Coins, error) {
	currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	total := sdk.Coins{}
	for i := range positions {
		claimed, err := k.claimBaseRewards(ctx, &positions[i], currentRatio)
		if err != nil {
			return sdk.Coins{}, err
		}
		total = total.Add(claimed...)

		if err := k.setPosition(ctx, positions[i]); err != nil {
			return sdk.Coins{}, err
		}
	}

	return total, nil
}

// claimBaseRewards calculates and sends a position's accrued base rewards.
// reward = DelegatedShares * (currentRatio - BaseRewardsPerShare)
func (k Keeper) claimBaseRewards(ctx context.Context, pos *types.Position, currentRatio sdk.DecCoins) (sdk.Coins, error) {
	if !pos.IsDelegated() {
		return sdk.Coins{}, nil
	}

	delta := currentRatio.Sub(pos.BaseRewardsPerShare)
	pos.UpdateBaseRewardsPerShare(currentRatio)

	if delta.IsAnyNegative() {
		k.logger(ctx).Error("base rewards per share is negative, skipping claim", "position", pos.String())
		return sdk.Coins{}, nil
	}

	if delta.IsZero() {
		return sdk.Coins{}, nil
	}

	posRewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()
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

func (k Keeper) claimBonusRewardsForPositions(ctx context.Context, positions []types.Position, forceAccrue bool) (sdk.Coins, error) {
	tierCache := make(map[uint32]types.Tier)
	total := sdk.NewCoins()

	for i := range positions {
		tier, ok := tierCache[positions[i].TierId]
		if !ok {
			var err error
			tier, err = k.getTier(ctx, positions[i].TierId)
			if err != nil {
				return sdk.Coins{}, err
			}
			tierCache[positions[i].TierId] = tier
		}

		bonus, err := k.claimBonusRewards(ctx, &positions[i], tier, forceAccrue)
		if err != nil {
			return sdk.Coins{}, err
		}

		total = total.Add(bonus...)

		if err := k.setPosition(ctx, positions[i]); err != nil {
			return sdk.Coins{}, err
		}
	}

	return total, nil
}

// claimBonusRewards calculates and pays the bonus for a position from the rewards pool.
// When forceAccrue is true, bonus is settled regardless of validator bonded status.
func (k Keeper) claimBonusRewards(ctx context.Context, pos *types.Position, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
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

	var bonus math.Int
	if forceAccrue {
		bonus = k.calculateBonusRaw(*pos, val, tier, blockTime)
	} else {
		bonus = k.calculateBonus(*pos, val, tier, blockTime)
	}

	accrualEnd := blockTime
	if pos.CompletedExitLockDuration(blockTime) {
		accrualEnd = pos.ExitUnlockAt
	}

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
