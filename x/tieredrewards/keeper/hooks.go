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

// AfterValidatorBeginUnbonding records an UNBOND event.
// Bonus for the pre-unbond segment is preserved via the snapshot rate.
func (h Hooks) AfterValidatorBeginUnbonding(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	count, err := h.k.getPositionCountForValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	tokensPerShare, err := h.k.getTokensPerShare(ctx, valAddr)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_, err = h.k.appendValidatorEvent(ctx, valAddr, types.ValidatorEvent{
		Height:         sdkCtx.BlockHeight(),
		Timestamp:      sdkCtx.BlockTime(),
		EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_UNBOND,
		TokensPerShare: tokensPerShare,
		ReferenceCount: count,
	})
	return err
}

// AfterValidatorBonded records a BOND event.
func (h Hooks) AfterValidatorBonded(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	count, err := h.k.getPositionCountForValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	tokensPerShare, err := h.k.getTokensPerShare(ctx, valAddr)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_, err = h.k.appendValidatorEvent(ctx, valAddr, types.ValidatorEvent{
		Height:         sdkCtx.BlockHeight(),
		Timestamp:      sdkCtx.BlockTime(),
		EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_BOND,
		TokensPerShare: tokensPerShare,
		ReferenceCount: count,
	})
	return err
}

// AfterValidatorRemoved cleans up validator reward ratio and events.
func (h Hooks) AfterValidatorRemoved(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	if err := h.k.clearValidatorRewardRatio(ctx, valAddr); err != nil {
		h.k.logger(ctx).Error("failed to cleanup validator reward ratio on validator removal", "validator", valAddr.String(), "error", err)
	}
	if err := h.k.deleteAllValidatorEvents(ctx, valAddr); err != nil {
		h.k.logger(ctx).Error("failed to cleanup validator events on validator removal", "validator", valAddr.String(), "error", err)
	}
	return nil
}

// BeforeValidatorSlashed records a SLASH event.
// The distribution module handles slash accounting for all delegators
// (including the tier module pool) via ValidatorSlashEvent records.
func (h Hooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction sdkmath.LegacyDec) error {
	count, err := h.k.getPositionCountForValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	tokensPerShare, err := h.k.getTokensPerShare(ctx, valAddr)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_, err = h.k.appendValidatorEvent(ctx, valAddr, types.ValidatorEvent{
		Height:         sdkCtx.BlockHeight(),
		Timestamp:      sdkCtx.BlockTime(),
		EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
		TokensPerShare: tokensPerShare,
		ReferenceCount: count,
	})
	return err
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
