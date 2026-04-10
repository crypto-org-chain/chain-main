package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// settleRewardsForPositions settles base and bonus rewards for a batch of
// positions on the same validator. It is designed for hook paths (slash,
// unbonding, bonding) where the caller must proceed even if the bonus pool
// is insufficient. When the bonus pool cannot cover a position's accrued
// bonus, the checkpoint is still advanced and the position persisted so the
// accrual window is consumed — the unpaid bonus is forfeited.
func (k Keeper) settleRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position, forceAccrue bool) error {
	currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return err
	}

	validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return err
	}

	tierCache := make(map[uint32]types.Tier)

	for i := range positions {
		if _, err := k.claimBaseRewards(ctx, &positions[i], currentRatio); err != nil {
			return err
		}

		tier, ok := tierCache[positions[i].TierId]
		if !ok {
			tier, err = k.getTier(ctx, positions[i].TierId)
			if err != nil {
				return err
			}
			tierCache[positions[i].TierId] = tier
		}

		_, err := k.claimBonusRewards(ctx, &positions[i], validator, tier, forceAccrue)
		if err != nil {
			if errors.Is(err, types.ErrInsufficientBonusPool) {
				k.logger(ctx).Error(err.Error())
			} else {
				return err
			}
		}
		// Persist regardless of whether bonus was paid.
		if err := k.setPosition(ctx, positions[i]); err != nil {
			return err
		}
	}

	return nil
}

// claimRewardsAndUpdatePositionsForTier claims base and bonus rewards for all delegated
// positions in the given tier.
func (k Keeper) claimRewardsAndUpdatePositionsForTier(ctx context.Context, tierId uint32) error {
	positions, err := k.getPositionsByTier(ctx, tierId)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	// Group delegated positions by validator because updateBaseRewardsPerShare
	// must be called once per validator batch.
	byValidator := make(map[string][]types.Position)
	for _, pos := range positions {
		if !pos.IsDelegated() {
			continue
		}
		byValidator[pos.Validator] = append(byValidator[pos.Validator], pos)
	}

	for valAddrStr, valPositions := range byValidator {
		valAddr, err := sdk.ValAddressFromBech32(valAddrStr)
		if err != nil {
			return err
		}

		currentRatio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
		if err != nil {
			return err
		}

		validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return err
		}

		tier, err := k.getTier(ctx, tierId)
		if err != nil {
			return err
		}

		for i := range valPositions {
			if _, err := k.claimBaseRewards(ctx, &valPositions[i], currentRatio); err != nil {
				return err
			}
			if _, err := k.claimBonusRewards(ctx, &valPositions[i], validator, tier, false); err != nil {
				return err
			}
			if err := k.setPosition(ctx, valPositions[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

// claimRewardsForPosition claims base and bonus rewards for a single position.
// positions in the given tier.
// Returns:
//   - position: the updated position with reward checkpoints advanced;
//   - base: base rewards paid to the owner for this position in this call;
//   - bonus: bonus rewards paid to the owner for this position in this call;
func (k Keeper) claimRewardsForPosition(ctx context.Context, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	if !pos.IsDelegated() {
		return pos, sdk.NewCoins(), sdk.NewCoins(), nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

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

	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	bonus, err := k.claimBonusRewards(ctx, &pos, val, tier, false)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	return pos, base, bonus, nil
}

// estimateRewardsForPosition calculates pending base and bonus rewards for a position
// without applying payouts or persisting position checkpoint updates.
func (k Keeper) estimateRewardsForPosition(ctx context.Context, pos types.Position) (sdk.Coins, sdk.Coins, error) {
	if !pos.IsDelegated() {
		return sdk.NewCoins(), sdk.NewCoins(), nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return nil, nil, err
	}

	currentRatio, err := k.estimateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return nil, nil, err
	}

	baseRewards := estimateBaseRewardsFromRatio(pos, currentRatio)

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return nil, nil, err
	}

	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return nil, nil, err
	}

	blockTime := sdk.UnwrapSDKContext(ctx).BlockTime()
	bonusAmount := k.bonusAccrualAmount(pos, val, tier, blockTime, false)
	bonusRewards, err := k.bonusCoinsIfPayable(ctx, pos, bonusAmount)
	if err != nil {
		return nil, nil, err
	}

	return baseRewards, bonusRewards, nil
}

func (k Keeper) estimateBaseRewardsPerShare(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, _ := sdkCtx.CacheContext()
	return k.updateBaseRewardsPerShare(cacheCtx, valAddr)
}

func estimateBaseRewardsFromRatio(pos types.Position, currentRatio sdk.DecCoins) sdk.Coins {
	delta, hasNegative := currentRatio.SafeSub(pos.BaseRewardsPerShare)
	if hasNegative {
		panic("negative base rewards per share delta")
	}
	if delta.IsZero() {
		return sdk.NewCoins()
	}

	posRewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()
	if posRewards.IsZero() {
		return sdk.NewCoins()
	}

	return posRewards
}

// claimBaseRewards calculates and sends a position's accrued base rewards.
// reward = DelegatedShares * (currentRatio - BaseRewardsPerShare)
func (k Keeper) claimBaseRewards(ctx context.Context, pos *types.Position, currentRatio sdk.DecCoins) (sdk.Coins, error) {
	if !pos.IsDelegated() {
		return sdk.NewCoins(), nil
	}

	delta, hasNegative := currentRatio.SafeSub(pos.BaseRewardsPerShare)
	if hasNegative {
		k.logger(ctx).Error(
			"difference in base rewards per share is negative, keeping previous checkpoint",
			"position", pos.String(),
			"current_ratio", currentRatio.String(),
			"delta", delta.String(),
		)
		panic("negative base rewards per share delta")
	}
	pos.UpdateBaseRewardsPerShare(currentRatio)

	if delta.IsZero() {
		return sdk.NewCoins(), nil
	}

	posRewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()
	if posRewards.IsZero() {
		return sdk.NewCoins(), nil
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return sdk.NewCoins(), err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, posRewards); err != nil {
		return sdk.NewCoins(), err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     posRewards,
	}); err != nil {
		return sdk.NewCoins(), err
	}

	return posRewards, nil
}

// claimBonusRewards calculates and pays the bonus for a position from the rewards pool.
// When forceAccrue is true, bonus is settled regardless of validator bonded status.
func (k Keeper) claimBonusRewards(ctx context.Context, pos *types.Position, val stakingtypes.Validator, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
	blockTime := sdk.UnwrapSDKContext(ctx).BlockTime()

	bonus := k.bonusAccrualAmount(*pos, val, tier, blockTime, forceAccrue)
	applyBonusAccrualCheckpoint(pos, blockTime)

	return k.sendBonusFromRewardsPool(ctx, *pos, bonus)
}
