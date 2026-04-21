package keeper

import (
	"context"
	"errors"

	"github.com/cosmos/gogoproto/proto"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"

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
		if _, err := k.claimBaseRewards(ctx, []*types.Position{&positions[i]}, positions[i].Owner, valAddr, currentRatio); err != nil {
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

		_, err := k.claimBonusRewards(ctx, []*types.Position{&positions[i]}, positions[i].Owner, validator, tier, forceAccrue)
		if err != nil {
			if errors.Is(err, types.ErrInsufficientBonusPool) {
				k.logger(ctx).Error("insufficient bonus pool for position",
					"position_id", positions[i].Id,
					"error", err.Error(),
				)
			} else {
				return err
			}
		}
		// Persist regardless of whether bonus was paid.
		// Use updatePosition since only reward checkpoints change.
		if err := k.updatePosition(ctx, positions[i]); err != nil {
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
		pos := positions[i]
		if !pos.IsDelegated() {
			continue
		}
		if _, seen := byValidator[pos.Validator]; !seen {
			validatorOrder = append(validatorOrder, pos.Validator)
		}
		byValidator[pos.Validator] = append(byValidator[pos.Validator], &pos)
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
			pos := valPositions[i]
			if _, err := k.claimBaseRewards(ctx, []*types.Position{pos}, pos.Owner, valAddr, currentRatio); err != nil {
				return err
			}
			if _, err := k.claimBonusRewards(ctx, []*types.Position{pos}, pos.Owner, validator, tier, false); err != nil {
				return err
			}
			// Use updatePosition since only reward checkpoints change.
			if err := k.updatePosition(ctx, *pos); err != nil {
				return err
			}
		}
	}

	return nil
}

// claimRewardsForPosition claims base and bonus rewards for a single position.
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

	base, err := k.claimBaseRewards(ctx, []*types.Position{&pos}, pos.Owner, valAddr, currentRatio)
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

	bonus, err := k.claimBonusRewards(ctx, []*types.Position{&pos}, pos.Owner, val, tier, false)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	return pos, base, bonus, nil
}

// claimRewardsForPositions claims base and bonus rewards for multiple positions for an owner,
// caching validator and tier lookups and batching bank sends to reduce gas.
// Fails atomically if any position cannot be claimed.
func (k Keeper) claimRewardsForPositions(ctx context.Context, owner string, positions []types.Position) (sdk.Coins, sdk.Coins, error) {
	type positionsByVal struct {
		positions []*types.Position
		ratio     sdk.DecCoins
		validator stakingtypes.Validator
	}

	type positionsByTier struct {
		positions []*types.Position
		tier      types.Tier
	}

	tierCache := make(map[uint32]types.Tier)
	var vals []string
	valGroups := make(map[string]*positionsByVal)

	for i := range positions {
		pos := &positions[i]
		// Defensive
		if !pos.IsOwner(owner) {
			return nil, nil, errorsmod.Wrapf(types.ErrNotPositionOwner, "position owner does not match owner, position: %s, owner: %s", pos.String(), owner)
		}

		if !pos.IsDelegated() {
			continue
		}

		valAddrStr := pos.Validator
		g, ok := valGroups[valAddrStr]
		if !ok {
			valAddr, err := sdk.ValAddressFromBech32(valAddrStr)
			if err != nil {
				return nil, nil, err
			}

			ratio, err := k.updateBaseRewardsPerShare(ctx, valAddr)
			if err != nil {
				return nil, nil, err
			}

			val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
			if err != nil {
				return nil, nil, err
			}

			g = &positionsByVal{ratio: ratio, validator: val}
			valGroups[valAddrStr] = g
			vals = append(vals, valAddrStr)
		}

		if _, ok := tierCache[pos.TierId]; !ok {
			tier, err := k.getTier(ctx, pos.TierId)
			if err != nil {
				return nil, nil, err
			}
			tierCache[pos.TierId] = tier
		}

		g.positions = append(g.positions, pos)
	}

	totalBase := sdk.NewCoins()
	totalBonus := sdk.NewCoins()

	for _, valAddrStr := range vals {
		g := valGroups[valAddrStr]

		valAddr, err := sdk.ValAddressFromBech32(valAddrStr)
		if err != nil {
			return nil, nil, err
		}

		base, err := k.claimBaseRewards(ctx, g.positions, owner, valAddr, g.ratio)
		if err != nil {
			return nil, nil, err
		}
		totalBase = totalBase.Add(base...)

		var tierIds []uint32
		tGroups := make(map[uint32]*positionsByTier)

		for _, pos := range g.positions {
			tg, ok := tGroups[pos.TierId]
			if !ok {
				tg = &positionsByTier{tier: tierCache[pos.TierId]}
				tGroups[pos.TierId] = tg
				tierIds = append(tierIds, pos.TierId)
			}
			tg.positions = append(tg.positions, pos)
		}

		for _, tierId := range tierIds {
			tg := tGroups[tierId]
			bonus, err := k.claimBonusRewards(ctx, tg.positions, owner, g.validator, tg.tier, false)
			if err != nil {
				return nil, nil, err
			}
			totalBonus = totalBonus.Add(bonus...)
		}
	}

	for i := range positions {
		pos := &positions[i]
		if !pos.IsDelegated() {
			continue
		}
		// Use updatePosition (no index diff) since claiming only modifies
		// reward checkpoints — owner, tier, and validator are unchanged.
		if err := k.updatePosition(ctx, *pos); err != nil {
			return nil, nil, err
		}
	}

	return totalBase, totalBonus, nil
}

// claimBaseRewards computes base rewards for the given positions, updates their
// BaseRewardsPerShare checkpoints, emits per-position EventBaseRewardsClaimed,
// and performs a single batched bank send for the total.
func (k Keeper) claimBaseRewards(ctx context.Context, positions []*types.Position, owner string, valAddr sdk.ValAddress, currentRatio sdk.DecCoins) (sdk.Coins, error) {
	total := sdk.NewCoins()
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	events := make([]proto.Message, 0, len(positions))

	for _, pos := range positions {
		// Defensive
		if !pos.IsOwner(owner) {
			return nil, errorsmod.Wrapf(types.ErrNotPositionOwner, "position owner does not match owner, position: %s, owner: %s", pos.String(), owner)
		}

		if !pos.IsDelegated() {
			continue
		}

		// Defensive
		if pos.Validator != valAddr.String() {
			return nil, errorsmod.Wrapf(types.ErrNotPositionValidator, "position validator does not match validator, position: %s, validator: %s", pos.String(), valAddr.String())
		}

		delta, hasNegative := currentRatio.SafeSub(pos.BaseRewardsPerShare)
		if hasNegative {
			k.logger(ctx).Error(
				"negative base rewards per share delta",
				"position", pos.String(),
				"current_ratio", currentRatio.String(),
				"delta", delta.String(),
			)
			panic("negative base rewards per share delta")
		}
		pos.UpdateBaseRewardsPerShare(currentRatio)

		if delta.IsZero() {
			continue
		}

		rewards, _ := delta.MulDecTruncate(pos.DelegatedShares).TruncateDecimal()
		if rewards.IsZero() {
			continue
		}

		events = append(events, &types.EventBaseRewardsClaimed{
			PositionId: pos.Id,
			Owner:      pos.Owner,
			Rewards:    rewards,
		})

		total = total.Add(rewards...)
	}

	if !total.IsZero() {
		ownerAddr, err := sdk.AccAddressFromBech32(owner)
		if err != nil {
			return nil, err
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, total); err != nil {
			return nil, err
		}
		if err := sdkCtx.EventManager().EmitTypedEvents(events...); err != nil {
			return nil, err
		}
	}

	return total, nil
}

// claimBonusRewards computes bonus rewards for the given positions, updates their
// LastBonusAccrual checkpoints, emits per-position EventBonusRewardsClaimed,
// and performs a single batched bank send from the rewards pool.
// When forceAccrue is true, bonus is settled regardless of validator bonded status.
func (k Keeper) claimBonusRewards(ctx context.Context, positions []*types.Position, owner string, val stakingtypes.Validator, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()
	total := sdk.NewCoins()

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}

	events := make([]proto.Message, 0, len(positions))

	for _, pos := range positions {
		// Defensive
		if !pos.IsOwner(owner) {
			return nil, errorsmod.Wrapf(types.ErrNotPositionOwner, "position owner does not match owner, position: %s, owner: %s", pos.String(), owner)
		}

		if !pos.IsDelegated() {
			continue
		}

		// Defensive
		if pos.Validator != val.OperatorAddress {
			return nil, errorsmod.Wrapf(types.ErrNotPositionValidator, "position validator does not match validator, position: %s, validator: %s", pos.String(), val.OperatorAddress)
		}

		// Defensive
		if pos.TierId != tier.Id {
			return nil, errorsmod.Wrapf(types.ErrNotPositionTier, "position tier does not match tier, position: %s, tier: %d", pos.String(), tier.Id)
		}

		bonus := k.bonusAccrualAmount(*pos, val, tier, blockTime, forceAccrue)
		applyBonusAccrualCheckpoint(pos, blockTime)

		if bonus.IsZero() {
			continue
		}

		bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, bonus))

		events = append(events, &types.EventBonusRewardsClaimed{
			PositionId: pos.Id,
			Owner:      pos.Owner,
			Rewards:    bonusCoins,
		})
		total = total.Add(bonusCoins...)
	}

	if !total.IsZero() {
		if _, err := k.bonusCoinsIfPayable(ctx, total); err != nil {
			return nil, err
		}

		ownerAddr, err := sdk.AccAddressFromBech32(owner)
		if err != nil {
			return nil, err
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, total); err != nil {
			return nil, err
		}

		if err := sdkCtx.EventManager().EmitTypedEvents(events...); err != nil {
			return nil, err
		}
	}

	return total, nil
}
