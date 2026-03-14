package keeper

import (
	"context"

	"errors"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
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

func (k Keeper) ClaimAllRewardsByValidator(ctx context.Context, valAddr sdk.ValAddress) error {
	positions, err := k.GetPositionsByValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	_, err = k.ClaimBaseRewardsForPositions(ctx, valAddr, positions)
	if err != nil {
		return err
	}

	_, err = k.ClaimBonusRewardsForPositions(ctx, positions)
	if err != nil && errors.Is(err, types.ErrInsufficientBonusPool) {
		k.Logger(ctx).Error("failed to claim bonus rewards due to insufficient funds in rewards pool",
			"validator", valAddr.String(),
			"error", err,
		)
		return nil
	}

	return err
}

// AfterValidatorBeginUnbonding is called when a validator transitions from bonded to unbonding.
// We settle all pending rewards (base + bonus) for each position on this validator,
// since no new base rewards will accrue after this point.
func (h Hooks) AfterValidatorBeginUnbonding(ctx context.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return h.k.ClaimAllRewardsByValidator(ctx, valAddr)
}
