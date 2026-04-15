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

	tier, err := k.getTier(ctx, tierId)
	if err != nil {
		return err
	}

	// Group delegated positions by validator
	var validatorOrder []string
	byValidator := make(map[string][]*types.Position)
	for i := range positions {
		if !positions[i].IsDelegated() {
			continue
		}
		if _, seen := byValidator[positions[i].Validator]; !seen {
			validatorOrder = append(validatorOrder, positions[i].Validator)
		}
		byValidator[positions[i].Validator] = append(byValidator[positions[i].Validator], &positions[i])
	}

	for _, valAddrStr := range validatorOrder {
		valPositions := byValidator[valAddrStr]
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

		for i := range valPositions {
			if _, err := k.claimBaseRewards(ctx, valPositions[i], currentRatio); err != nil {
				return err
			}
			if _, err := k.claimBonusRewards(ctx, valPositions[i], validator, tier, false); err != nil {
				return err
			}
			if err := k.setPosition(ctx, *valPositions[i]); err != nil {
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
