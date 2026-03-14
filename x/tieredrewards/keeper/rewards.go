package keeper

import (
	"context"
	"errors"
	"time"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
)

// delegateFromPosition delegates tokens from the tier module account to a validator on behalf of a position.
// Only bonded validators are allowed
// Returns the delegation shares received from the staking module.
func (k Keeper) delegateFromPosition(ctx context.Context, validator string, amount math.Int) (math.LegacyDec, error) {
	valAddr, err := sdk.ValAddressFromBech32(validator)
	if err != nil {
		return math.LegacyDec{}, sdkerrors.ErrInvalidAddress
	}
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
	if position.IsExiting() && blockTime.After(position.ExitUnlockAt) {
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

// ClaimBaseRewardsForPositions withdraws base staking rewards from distribution
// for the module's delegation to the given validator and distributes them
// proportionally to each position based on their share of the total delegation.
func (k Keeper) ClaimBaseRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position) (sdk.Coins, error) {
	rewards, err := k.withdrawDelegationRewards(ctx, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	if rewards.IsZero() || len(positions) == 0 {
		return rewards, nil
	}

	poolAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	delegation, err := k.stakingKeeper.GetDelegation(ctx, poolAddr, valAddr)
	if err != nil {
		return sdk.Coins{}, err
	}

	totalShares := delegation.Shares

	decRewards := sdk.NewDecCoinsFromCoins(rewards...)

	for _, pos := range positions {
		if err := k.ClaimBaseRewards(ctx, pos, decRewards, totalShares); err != nil {
			return sdk.Coins{}, err
		}
	}

	return rewards, nil
}

// ClaimBaseRewards calculates and sends a position's share of base rewards to the owner.
// Follows the same pattern as x/distribution: ratio first, then multiply with MulDecTruncate.
func (k Keeper) ClaimBaseRewards(ctx context.Context, pos types.Position, totalRewards sdk.DecCoins, totalShares math.LegacyDec) error {
	if totalShares.IsZero() {
		return nil
	}
	fraction := pos.DelegatedShares.QuoTruncate(totalShares)
	posRewards, _ := totalRewards.MulDecTruncate(fraction).TruncateDecimal()

	if posRewards.IsZero() {
		return nil
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return err
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, posRewards)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.EventManager().EmitTypedEvent(&types.EventBaseRewardsClaimed{
		PositionId: pos.Id,
		Owner:      pos.Owner,
		Amount:     posRewards,
	})
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
	if pos.IsExiting() && blockTime.After(pos.ExitUnlockAt) {
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
