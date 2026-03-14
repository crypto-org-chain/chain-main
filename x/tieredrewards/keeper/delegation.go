package keeper

import (
	"context"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
