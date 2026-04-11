package keeper

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// validatorProvider captures the staking query needed to convert delegated
// shares into governance voting power.
type validatorProvider interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
}

// isTierGovernanceEligiblePosition defines the tier-governance inclusion rule.
// A position contributes governance power only when it is delegated and has
// non-zero economic value.
func isTierGovernanceEligiblePosition(pos types.Position) bool {
	return pos.IsDelegated() && !pos.Amount.IsZero() && pos.DelegatedShares.IsPositive()
}

func tierVotingPowerForPosition(
	ctx context.Context,
	sk validatorProvider,
	pos types.Position,
	validatorCache map[string]stakingtypes.Validator,
) (math.LegacyDec, error) {
	if !isTierGovernanceEligiblePosition(pos) {
		return math.LegacyZeroDec(), nil
	}

	val, ok := validatorCache[pos.Validator]
	if !ok {
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return math.LegacyZeroDec(), nil
		}

		val, err = sk.GetValidator(ctx, valAddr)
		if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
			return math.LegacyZeroDec(), nil
		}
		if err != nil {
			return math.LegacyZeroDec(), err
		}

		if validatorCache != nil {
			validatorCache[pos.Validator] = val
		}
	}

	if val.DelegatorShares.IsZero() {
		return math.LegacyZeroDec(), nil
	}

	return val.TokensFromShares(pos.DelegatedShares), nil
}
