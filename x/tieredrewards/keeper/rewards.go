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

func (k Keeper) undelegate(ctx context.Context, valAddr sdk.ValAddress, shares math.LegacyDec) (time.Time, math.Int, uint64, error) {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	completionTime, returnAmount, unbondingId, err := k.stakingKeeper.Undelegate(ctx, moduleAddr, valAddr, shares)
	if err != nil {
		return time.Time{}, math.Int{}, 0, err
	}
	return completionTime, returnAmount, unbondingId, nil
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

// collectValidatorDelegationRewards pulls accumulated staking distribution rewards from
// x/distribution into the tier module account for the delegation to valAddr.
func (k Keeper) collectValidatorDelegationRewards(ctx context.Context, valAddr sdk.ValAddress, currentBlockHeight uint64) (rewards sdk.Coins, collected bool, err error) {
	lastWithdrawalBlock, err := k.ValidatorRewardsLastWithdrawalBlock.Get(ctx, valAddr)
	if err == nil && lastWithdrawalBlock == currentBlockHeight {
		return sdk.Coins{}, false, nil
	}
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return sdk.Coins{}, false, err
	}

	rewards, err = k.withdrawDelegationRewards(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, false, err
	}

	if err := k.ValidatorRewardsLastWithdrawalBlock.Set(ctx, valAddr, currentBlockHeight); err != nil {
		return sdk.Coins{}, false, err
	}

	return rewards, true, nil
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

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rewards, collected, err := k.collectValidatorDelegationRewards(ctx, valAddr, uint64(sdkCtx.BlockHeight()))
	if err != nil {
		return sdk.DecCoins{}, err
	}
	if !collected || rewards.IsZero() {
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

// slash updates a position's post-slash token amount from validator shares.
// LegacyDec rounding may differ from SDK accounting by up to 1 basecro.
// pos.Amount is reconciled with the SDK return value during TierUndelegate.
func (k Keeper) slash(pos *types.Position, validator stakingtypes.Validator, fraction math.LegacyDec) {
	postSlashTokens := validator.TokensFromShares(pos.DelegatedShares).Mul(math.LegacyOneDec().Sub(fraction)).TruncateInt()
	pos.UpdateAmount(math.MaxInt(postSlashTokens, math.ZeroInt()))
}

// slashPositionByUnbondingId subtracts slashAmount from a mapped position.
// No-op if unbondingId is not mapped to a tier position.
func (k Keeper) slashPositionByUnbondingId(ctx context.Context, unbondingId uint64, slashAmount math.Int) error {
	positionId, err := k.UnbondingDelegationMappings.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.getPosition(ctx, positionId)
	if errors.Is(err, types.ErrPositionNotFound) {
		// Stale mapping after position lifecycle completion.
		return k.deleteUnbondingPositionMapping(ctx, unbondingId)
	}
	if err != nil {
		return err
	}

	newAmount := math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt())
	pos.UpdateAmount(newAmount)

	return k.setPosition(ctx, pos)
}

// slashRedelegationPosition reduces both Amount and DelegatedShares for
// a position mapped to the given redelegation unbonding ID.
func (k Keeper) slashRedelegationPosition(ctx context.Context, unbondingId uint64, slashAmount math.Int, shareBurnt math.LegacyDec) error {
	positionId, err := k.RedelegationMappings.Get(ctx, unbondingId)
	if errors.Is(err, collections.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	pos, err := k.getPosition(ctx, positionId)
	if errors.Is(err, types.ErrPositionNotFound) {
		return k.deleteRedelegationPositionMapping(ctx, unbondingId)
	}
	if err != nil {
		return err
	}

	newAmount := math.MaxInt(pos.Amount.Sub(slashAmount), math.ZeroInt())
	pos.UpdateAmount(newAmount)

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
//
// Returns:
//   - baseRewards: total base (staking distribution) rewards paid to position owners
//     for the given positions in this call;
//   - bonusRewards: total bonus rewards paid from the rewards pool for those positions;
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

// claimAndRefreshPosition claims rewards for a single position and returns the
// updated in-memory value so callers can finish their mutation and persist once.
//
// Returns:
//   - refreshed: the updated position with reward checkpoints advanced;
//   - base: base rewards paid to the owner for this position in this call;
//   - bonus: bonus rewards paid to the owner for this position in this call;
func (k Keeper) claimAndRefreshPosition(ctx context.Context, valAddr sdk.ValAddress, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	base, err := k.claimBaseRewards(ctx, &pos, currentRatio)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	bonus, err := k.claimBonusRewards(ctx, &pos, tier, false)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	return pos, base, bonus, nil
}

// claimBaseRewardsForPositions settles staking distribution (base) rewards for each
// position delegated to valAddr. It first updates the validator's cumulative
// rewards-per-share from x/distribution, then for each position computes the
// owner's share, transfers coins from the module account, updates checkpoints,
// and persists the position.
//
// Returns:
//   - total: the sum of base coins paid to owners across all positions in this call;
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

// claimBonusRewardsForPositions settles bonus rewards for each position, loading tiers
// as needed and persisting updated positions after each claim.
// When forceAccrue is true, bonus is calculated regardless of validator bonded status
// (see claimBonusRewards).
//
// Returns:
//   - total: the sum of bonus coins paid to owners across all positions in this call;
func (k Keeper) claimBonusRewardsForPositions(ctx context.Context, positions []types.Position, forceAccrue bool) (sdk.Coins, error) {
	tierCache := make(map[uint32]types.Tier)
	total := sdk.NewCoins()
	var firstErr error

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
			// Hooks tolerate an insufficient bonus pool so validator lifecycle and
			// slashing can proceed. Persist the advanced checkpoint before returning
			// the error so callers that swallow it do not later reprice the same
			// accrual window against different validator state.
			if errors.Is(err, types.ErrInsufficientBonusPool) {
				if setErr := k.setPosition(ctx, positions[i]); setErr != nil {
					return sdk.Coins{}, setErr
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			return sdk.Coins{}, err
		}

		total = total.Add(bonus...)

		if err := k.setPosition(ctx, positions[i]); err != nil {
			return sdk.Coins{}, err
		}
	}

	if firstErr != nil {
		return total, firstErr
	}

	return total, nil
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
func (k Keeper) bonusAccrualAmount(pos *types.Position, val stakingtypes.Validator, tier types.Tier, blockTime time.Time, forceAccrue bool) math.Int {
	if forceAccrue {
		return k.calculateBonusRaw(*pos, val, tier, blockTime)
	}
	return k.calculateBonus(*pos, val, tier, blockTime)
}

// sendBonusFromRewardsPool checks the rewards pool, transfers bonus to the owner, and emits EventBonusRewardsClaimed.
func (k Keeper) sendBonusFromRewardsPool(ctx context.Context, pos *types.Position, bonus math.Int) (sdk.Coins, error) {
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

// claimBonusRewards calculates and pays the bonus for a position from the rewards pool.
// When forceAccrue is true, bonus is settled regardless of validator bonded status.
func (k Keeper) claimBonusRewards(ctx context.Context, pos *types.Position, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
	blockTime := sdk.UnwrapSDKContext(ctx).BlockTime()

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return sdk.Coins{}, err
	}

	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	bonus := k.bonusAccrualAmount(pos, val, tier, blockTime, forceAccrue)
	applyBonusAccrualCheckpoint(pos, blockTime)

	if bonus.IsZero() {
		return sdk.Coins{}, nil
	}

	return k.sendBonusFromRewardsPool(ctx, pos, bonus)
}
