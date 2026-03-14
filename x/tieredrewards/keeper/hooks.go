package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Hooks wraps the Keeper to implement staking hooks.
type Hooks struct {
	k Keeper
}

var _ stakingtypes.StakingHooks = Hooks{}

// Hooks returns the staking hooks for the tieredrewards module.
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterValidatorBeginUnbonding is called when a validator transitions from bonded to unbonding.
// We settle all pending rewards (base + bonus) for each position on this validator,
// since no new base rewards will accrue after this point.
func (h Hooks) AfterValidatorBeginUnbonding(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	positions, err := h.k.GetPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	return h.k.ClaimRewardsForPositions(ctx, valAddr, positions)
}

// BeforeValidatorSlashed is called before a validator is slashed.
// We will first claim any pending rewards for all positions on this validator.
// It is possible for a validator to be slashed multiple times, even when the validator is already unbonding/unbonded,
// so there is a need to claim rewards here independently from AfterValidatorBeginUnbonding.
// We will then reduce Amount on all positions delegated to this validator by the slash fraction.
// This keeps bonus APY and claim amounts consistent with the actual token value.
func (h Hooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction sdkmath.LegacyDec) error {
	positions, err := h.k.GetPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	err = h.k.ClaimRewardsForPositions(ctx, valAddr, positions)
	if err != nil {
		return err
	}

	return h.k.slashPositions(ctx, positions, fraction)

}

// AfterValidatorBonded is called when a validator transitions to bonded.
// We reset LastBonusAccrual for all positions on this validator to the current block time,
// so bonus only accrues from when the validator is bonded again.
// Without this, positions would over-claim bonus for the period the validator was unbonding.
func (h Hooks) AfterValidatorBonded(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	positions, err := h.k.GetPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	for _, pos := range positions {
		pos.LastBonusAccrual = blockTime
		if err := h.k.SetPosition(ctx, pos); err != nil {
			return err
		}
	}

	return nil
}

// --- No-op hooks ---

func (h Hooks) AfterValidatorCreated(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorModified(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) AfterValidatorRemoved(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
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

func (h Hooks) AfterUnbondingInitiated(_ context.Context, _ uint64) error {
	return nil
}
