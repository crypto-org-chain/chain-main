package keeper

import (
	"context"

	"github.com/cosmos/gogoproto/proto"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// claimBaseRewards claims the outstanding base rewards held
// by the given position's delegation for a single validator.
// This assumes that the delegation withdrawAddress has been set to the position's owner address.
func (k Keeper) claimBaseRewards(ctx context.Context, pos types.Position, valAddr sdk.ValAddress) (sdk.Coins, error) {
	delAddr, err := sdk.AccAddressFromBech32(pos.DelegatorAddress)
	if err != nil {
		return nil, err
	}
	rewards, err := k.distributionKeeper.WithdrawDelegationRewards(ctx, delAddr, valAddr)
	if err != nil {
		return nil, err
	}
	if rewards.IsZero() {
		return rewards, nil
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Rewards:    rewards,
	}); err != nil {
		return nil, err
	}
	return rewards, nil
}

// claimRewardsAndUpdateTierPositions claims base and bonus rewards for all
// delegated positions in the given tier.
func (k Keeper) claimRewardsAndUpdateTierPositions(ctx context.Context, tierId uint32) error {
	positions, err := k.getPositionsByTier(ctx, tierId)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	valCache := make(map[string]sdk.ValAddress)

	for i := range positions {
		pos := &positions[i]
		if !pos.IsDelegated() {
			continue
		}

		valAddr, ok := valCache[pos.Validator]
		if !ok {
			valAddr, err = sdk.ValAddressFromBech32(pos.Validator)
			if err != nil {
				return err
			}
			valCache[pos.Validator] = valAddr
		}

		if _, err := k.claimBaseRewards(ctx, *pos, valAddr); err != nil {
			return err
		}
		if _, err := k.processEventsAndClaimBonus(ctx, pos, valAddr); err != nil {
			return err
		}
		// setPositionUnsafe to reduce gas consumption
		if err := k.setPositionUnsafe(ctx, *pos); err != nil {
			return err
		}
	}

	return nil
}

// claimRewards claims base and bonus rewards for a single position.
// Returns:
//   - position: the updated position with reward checkpoints advanced;
//   - base: base rewards paid to the owner for this position in this call;
//   - bonus: bonus rewards paid to the owner for this position in this call;
func (k Keeper) claimRewards(ctx context.Context, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	if !pos.IsDelegated() {
		return pos, sdk.NewCoins(), sdk.NewCoins(), nil
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	base, err := k.claimBaseRewards(ctx, pos, valAddr)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	bonus, err := k.processEventsAndClaimBonus(ctx, &pos, valAddr)
	if err != nil {
		return types.Position{}, nil, nil, err
	}

	return pos, base, bonus, nil
}

// claimRewardsAndUpdatesPositions claims base and bonus rewards for multiple positions.
func (k Keeper) claimRewardsAndUpdatesPositions(ctx context.Context, owner string, positions []types.Position) (sdk.Coins, sdk.Coins, error) {
	valCache := make(map[string]sdk.ValAddress)

	totalBase := sdk.NewCoins()
	totalBonus := sdk.NewCoins()

	for i := range positions {
		pos := &positions[i]
		// Defensive
		if !pos.IsOwner(owner) {
			return nil, nil, errorsmod.Wrapf(types.ErrNotPositionOwner, "position owner does not match owner, position: %s, owner: %s", pos.String(), owner)
		}

		if !pos.IsDelegated() {
			continue
		}

		valAddr, ok := valCache[pos.Validator]
		if !ok {
			var err error
			valAddr, err = sdk.ValAddressFromBech32(pos.Validator)
			if err != nil {
				return nil, nil, err
			}
			valCache[pos.Validator] = valAddr
		}

		base, err := k.claimBaseRewards(ctx, *pos, valAddr)
		if err != nil {
			return nil, nil, err
		}
		totalBase = totalBase.Add(base...)

		bonus, err := k.processEventsAndClaimBonus(ctx, pos, valAddr)
		if err != nil {
			return nil, nil, err
		}
		totalBonus = totalBonus.Add(bonus...)

		// setPositionUnsafe to reduce gas consumption
		if err := k.setPositionUnsafe(ctx, *pos); err != nil {
			return nil, nil, err
		}
	}

	return totalBase, totalBonus, nil
}

// processEventsAndClaimBonus processes pending validator events for a position
// and computes the bonus rewards owed. It walks through events since the position's
// LastEventSeq, computing bonus for each bonded segment using snapshot rates.
//
// Returns the total bonus coins paid to the owner.
func (k Keeper) processEventsAndClaimBonus(ctx context.Context, pos *types.Position, valAddr sdk.ValAddress) (sdk.Coins, error) {
	// Rewards should have been claimed before undelegation
	if !pos.IsDelegated() {
		return sdk.NewCoins(), nil
	}

	// Defensive
	if pos.Validator != valAddr.String() {
		return nil, errorsmod.Wrapf(types.ErrNotPositionValidator, "position validator does not match validator, position: %s, validator: %s", pos.String(), valAddr.String())
	}

	events, err := k.getValidatorEventsSince(ctx, valAddr, pos.LastEventSeq)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	totalBonus := math.ZeroInt()
	// Use the persisted bonded state from the last replay, not a hardcoded default.
	// This prevents overpaying bonus for unbonded gaps between claims.
	bonded := pos.LastKnownBonded
	segmentStart := pos.LastBonusAccrual

	tier, err := k.getTier(ctx, pos.TierId)
	if err != nil {
		return nil, err
	}

	for _, entry := range events {
		evt := entry.Event

		if bonded {
			// Compute bonus for the bonded segment [segmentStart, eventTime]
			// using the snapshot rate at the event.
			bonus := k.computeSegmentBonus(pos, tier, segmentStart, evt.Timestamp, evt.TokensPerShare)
			totalBonus = totalBonus.Add(bonus)
		}

		// Update bonded state based on event type.
		switch evt.EventType {
		case types.ValidatorEventType_VALIDATOR_EVENT_TYPE_UNBOND:
			bonded = false
		case types.ValidatorEventType_VALIDATOR_EVENT_TYPE_BOND:
			bonded = true
		case types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH:
			// Slash doesn't change bonded state.
		}

		segmentStart = evt.Timestamp
		pos.UpdateLastEventSeq(entry.Seq)

		// Decrement reference count.
		if err := k.decrementEventRefCount(ctx, valAddr, entry.Seq); err != nil {
			return nil, err
		}
	}

	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return nil, err
	}
	// Defensive: validator bond status check
	if bonded && val.IsBonded() {
		currentRate, err := k.getTokensPerShare(ctx, valAddr)
		if err != nil {
			return nil, err
		}
		bonus := k.computeSegmentBonus(pos, tier, segmentStart, blockTime, currentRate)
		totalBonus = totalBonus.Add(bonus)
	}

	applyBonusAccrualCheckpoint(pos, blockTime)
	// Persist the bonded state so the next replay starts correctly.
	pos.UpdateLastKnownBonded(bonded)

	if totalBonus.IsZero() {
		return sdk.NewCoins(), nil
	}

	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}

	bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, totalBonus))

	if err := k.sufficientBonusPoolBalance(ctx, bonusCoins); err != nil {
		return nil, err
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return nil, err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RewardsPoolName, ownerAddr, bonusCoins); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBonusRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Rewards:    bonusCoins,
	}); err != nil {
		return nil, err
	}

	return bonusCoins, nil
}

// claimBaseRewardsOld computes base rewards for the given positions, updates their
// BaseRewardsPerShare checkpoints, emits per-position EventBaseRewardsClaimed,
// and performs a single batched bank send for the total.
func (k Keeper) claimBaseRewardsOld(ctx context.Context, positions []*types.Position, owner string, valAddr sdk.ValAddress, currentRatio sdk.DecCoins) (sdk.Coins, error) {
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
