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

// AfterValidatorRemoved cleans up validator events and seq if no leftover
// events reference them. If leftover events exist (positions not yet claimed),
// nothing is cleaned — the seq is still needed for future claims.
func (h Hooks) AfterValidatorRemoved(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	has, err := h.k.hasValidatorEvents(ctx, valAddr)
	if err != nil {
		return err
	}
	if has {
		h.k.logger(ctx).Error("leftover validator events found on validator removal, skipping cleanup", "validator", valAddr.String())
		return nil
	}

	if err := h.k.deleteValidatorEventSeq(ctx, valAddr); err != nil {
		h.k.logger(ctx).Error("failed to cleanup validator event sequence on validator removal", "validator", valAddr.String(), "error", err)
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

// BeforeRedelegationSlashed fires before SDK's Unbond in SlashRedelegation.
// Routes to slashRedelegationPosition via the unbondingId → positionId mapping
// so bonus settlement can run against pre-slash shares.
func (h Hooks) BeforeRedelegationSlashed(ctx context.Context, unbondingID uint64, sharesToUnbond sdkmath.LegacyDec) error {
	return h.k.slashRedelegationPosition(ctx, unbondingID, sharesToUnbond)
}

// AfterRedelegationCompleted removes the redelegation-mapping entries for each
// unbondingId that just matured.
func (h Hooks) AfterRedelegationCompleted(ctx context.Context, _ sdk.AccAddress, _, _ sdk.ValAddress, completedIds []uint64) error {
	for _, id := range completedIds {
		has, err := h.k.RedelegationMappings.Has(ctx, id)
		if err != nil {
			return err
		}
		if !has {
			continue
		}
		if err := h.k.deleteRedelegationMapping(ctx, id); err != nil {
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
