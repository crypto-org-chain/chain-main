package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
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

func (h Hooks) claimAllRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position, forceAccrue bool) error {
	_, _, err := h.k.claimRewardsForPositions(ctx, valAddr, positions, forceAccrue)
	if err != nil && errors.Is(err, types.ErrInsufficientBonusPool) {
		h.k.logger(ctx).Error("failed to claim bonus rewards due to insufficient funds in rewards pool",
			"validator", valAddr.String(),
			"error", err,
		)
		return nil
	}
	return err
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

	return h.claimAllRewardsForPositions(ctx, valAddr, positions, true)
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

	err = h.claimAllRewardsForPositions(ctx, valAddr, positions, false)
	if err != nil {
		return err
	}

	// Re-fetch after claiming so slashPositions operates on latest store state.
	positions, err = h.k.getPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}

	return h.k.slashPositions(ctx, valAddr, positions, fraction)
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

func (h Hooks) AfterSlashUnbondingDelegation(ctx context.Context, unbondingId uint64, slashAmount sdkmath.Int) error {
	return h.k.slashPositionByUnbondingId(ctx, unbondingId, slashAmount)
}

func (h Hooks) AfterSlashUnbondingRedelegation(ctx context.Context, unbondingId uint64, slashAmount sdkmath.Int) error {
	return h.k.slashPositionByUnbondingId(ctx, unbondingId, slashAmount)
}

// AfterSlashRedelegation updates DelegatedShares when an active destination
// delegation is slashed via redelegation.
func (h Hooks) AfterSlashRedelegation(ctx context.Context, unbondingId uint64, slashAmount sdkmath.Int, shareBurnt sdkmath.LegacyDec) error {
	return h.k.slashRedelegationPosition(ctx, unbondingId, slashAmount, shareBurnt)
}

func (h Hooks) AfterUnbondingInitiated(_ context.Context, _ uint64) error {
	return nil
}

// No-op hooks.

func (h Hooks) AfterValidatorCreated(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorModified(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) AfterValidatorRemoved(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	if err := h.k.ValidatorRewardRatio.Remove(ctx, valAddr); err != nil && !errors.Is(err, collections.ErrNotFound) {
		h.k.logger(ctx).Error("failed to cleanup validator reward ratio on validator removal", "validator", valAddr.String(), "error", err)
	}
	if err := h.k.ValidatorRewardsLastWithdrawalBlock.Remove(ctx, valAddr); err != nil && !errors.Is(err, collections.ErrNotFound) {
		h.k.logger(ctx).Error("failed to cleanup validator withdrawal marker on validator removal", "validator", valAddr.String(), "error", err)
	}
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
