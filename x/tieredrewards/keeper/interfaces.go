package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	addresscodec "cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TierVotingPowerProvider is the interface the custom gov tally needs from the
// tiered rewards module. It is intentionally narrow: the tally only needs the
// per-position DelegatedShares to compute shares-to-tokens power and to deduct
// from the validator's DelegatorDeductions. GetVotingPowerForAddress is
// available on the concrete Keeper (used by the gRPC query) but is not part of
// this interface.
type TierVotingPowerProvider interface {
	// GetActiveDelegatedPositionsByOwner returns all positions owned by voter
	// that are currently delegated and have NOT triggered an exit. The tally
	// function uses these to deduct each position's DelegatedShares from the
	// corresponding validator's DelegatorDeductions, preventing the module
	// account's delegation from being double-counted in the validator second pass.
	GetActiveDelegatedPositionsByOwner(ctx context.Context, voter sdk.AccAddress) ([]types.Position, error)
}

// GovTallyStakingKeeper is the subset of staking keeper needed by the custom
// tally function (matches gov's types.StakingKeeper).
type GovTallyStakingKeeper interface {
	ValidatorAddressCodec() addresscodec.Codec
	IterateDelegations(ctx context.Context, delegator sdk.AccAddress, fn func(index int64, delegation stakingtypes.DelegationI) (stop bool)) error
}

// GovTallyAccountKeeper is the subset of account keeper needed by the custom
// tally function.
type GovTallyAccountKeeper interface {
	AddressCodec() addresscodec.Codec
}
