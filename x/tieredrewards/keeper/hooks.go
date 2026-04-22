package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Hooks wraps the Keeper to implement staking hooks.
type Hooks struct {
	k Keeper
}

var _ stakingtypes.StakingHooks = Hooks{}

func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterValidatorBeginUnbonding settles all pending rewards for each position on
// this validator. forceAccrue=true is required because the SDK has already changed
// the validator status to Unbonding before firing this hook.
func (h Hooks) AfterValidatorBeginUnbonding(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	positions, err := h.k.getPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	_, err = h.k.settleRewardsForPositions(ctx, valAddr, positions, true)
	return err
}

// AfterValidatorBonded resets LastBonusAccrual for all positions on this
// validator so bonus only accrues from when the validator is bonded again.
func (h Hooks) AfterValidatorBonded(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	positions, err := h.k.getPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	for _, pos := range positions {
		pos.UpdateLastBonusAccrual(blockTime)
		if err := h.k.setPosition(ctx, pos); err != nil {
			return err
		}
	}

	return nil
}

// AfterValidatorRemoved cleans up validator reward ratio.
func (h Hooks) AfterValidatorRemoved(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	if err := h.k.clearValidatorRewardRatio(ctx, valAddr); err != nil {
		h.k.logger(ctx).Error("failed to cleanup validator reward ratio on validator removal", "validator", valAddr.String(), "error", err)
	}
	return nil
}

// BeforeValidatorSlashed claims pending rewards then reduces Amount on all
// positions by the slash fraction.
func (h Hooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction sdkmath.LegacyDec) error {
	positions, err := h.k.getPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	positions, err = h.k.settleRewardsForPositions(ctx, valAddr, positions, false)
	if err != nil {
		return err
	}

	if err := h.k.slashPositions(ctx, valAddr, positions, fraction); err != nil {
		return err
	}

	return h.k.updatePoolDelegationInfo(ctx, valAddr, fraction)
}

func (h Hooks) AfterUnbondingDelegationSlashed(ctx context.Context, unbondingId uint64, slashAmount sdkmath.Int) error {
	return h.k.slashPositionByUnbondingId(ctx, unbondingId, slashAmount)
}

func (h Hooks) AfterUnbondingRedelegationSlashed(ctx context.Context, unbondingId uint64, slashAmount sdkmath.Int) error {
	return h.k.slashPositionByUnbondingId(ctx, unbondingId, slashAmount)
}

// AfterRedelegationSlashed updates DelegatedShares when an active destination
// delegation is slashed via redelegation.
func (h Hooks) AfterRedelegationSlashed(ctx context.Context, unbondingId uint64, slashAmount sdkmath.Int, shareBurnt sdkmath.LegacyDec) error {
	return h.k.slashRedelegationPosition(ctx, unbondingId, slashAmount, shareBurnt)
}

func (h Hooks) AfterUnbondingCompleted(ctx context.Context, delAddr sdk.AccAddress, _ sdk.ValAddress, unbondingIds []uint64) error {
	return h.deleteCompletedPositionMappings(
		ctx,
		delAddr,
		unbondingIds,
		h.k.UnbondingDelegationMappings.Has,
		h.k.deleteUnbondingPositionMapping,
	)
}

func (h Hooks) AfterRedelegationCompleted(ctx context.Context, delAddr sdk.AccAddress, _, _ sdk.ValAddress, unbondingIds []uint64) error {
	return h.deleteCompletedPositionMappings(
		ctx,
		delAddr,
		unbondingIds,
		h.k.RedelegationMappings.Has,
		h.k.deleteRedelegationPositionMapping,
	)
}

func (h Hooks) deleteCompletedPositionMappings(
	ctx context.Context,
	delAddr sdk.AccAddress,
	unbondingIds []uint64,
	hasMapping func(context.Context, uint64) (bool, error),
	deleteMapping func(context.Context, uint64) error,
) error {
	poolAddr := h.k.accountKeeper.GetModuleAddress(types.ModuleName)
	if !delAddr.Equals(poolAddr) {
		return nil
	}
	for _, id := range unbondingIds {
		has, err := hasMapping(ctx, id)
		if err != nil {
			return err
		}
		if !has {
			continue
		}
		if err := deleteMapping(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// No-op hooks.

func (h Hooks) AfterUnbondingInitiated(_ context.Context, _ uint64) error {
	return nil
}

func (h Hooks) AfterValidatorCreated(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorModified(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationCreated(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationSharesModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationRemoved(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) AfterDelegationModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}
