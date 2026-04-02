package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	addresscodec "cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TierVotingPowerProvider is the interface the custom gov tally needs from
// the tiered rewards module.
type TierVotingPowerProvider interface {
	GetDelegatedPositionsByOwner(ctx context.Context, voter sdk.AccAddress) ([]types.Position, error)
}

// GovTallyStakingKeeper is the subset of staking keeper needed by the custom tally function.
type GovTallyStakingKeeper interface {
	ValidatorAddressCodec() addresscodec.Codec
	IterateDelegations(ctx context.Context, delegator sdk.AccAddress, fn func(index int64, delegation stakingtypes.DelegationI) (stop bool)) error
}

// GovTallyAccountKeeper is the subset of account keeper needed by the custom tally function.
type GovTallyAccountKeeper interface {
	AddressCodec() addresscodec.Codec
}
