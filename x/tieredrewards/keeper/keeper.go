package keeper

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper of the tieredrewards store.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService storetypes.KVStoreService
	authority    string

	Schema collections.Schema
	Params collections.Item[types.Params]

	Tiers collections.Map[uint32, types.Tier]

	// Positions
	Positions      collections.Map[uint64, types.Position]
	NextPositionId collections.Sequence

	// Secondary indexes
	PositionsByOwner     collections.KeySet[collections.Pair[sdk.AccAddress, uint64]]
	PositionsByTier      collections.KeySet[collections.Pair[uint32, uint64]]
	PositionsByValidator collections.KeySet[collections.Pair[sdk.ValAddress, uint64]]

	// Counters
	PositionCountByTier collections.Map[uint32, uint64]

	// Base reward tracking: cumulative rewards-per-share indexed by validator
	ValidatorRewardRatio collections.Map[sdk.ValAddress, types.ValidatorRewardRatio]

	// Unbonding tracking: maps staking unbonding IDs to tier position IDs.
	// Populated in message handlers (MsgTierUndelegate, MsgTierRedelegate) after
	// calling staking Undelegate/BeginRedelegate which returns the unbonding ID.
	// Used by slash hooks to find the affected position when unbonding delegations
	// or redelegations are slashed.
	UnbondingIdToPositionId collections.Map[uint64, uint64]

	mintKeeper         types.MintKeeper
	stakingKeeper      types.StakingKeeper
	accountKeeper      types.AccountKeeper
	bankKeeper         types.BankKeeper
	distributionKeeper types.DistributionKeeper
}

// NewKeeper creates a new tieredrewards Keeper instance.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService storetypes.KVStoreService,
	authority string,
	mintKeeper types.MintKeeper,
	stakingKeeper types.StakingKeeper,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	distributionKeeper types.DistributionKeeper,
) Keeper {
	// ensure base rewards pool module account is set
	if addr := accountKeeper.GetModuleAddress(types.RewardsPoolName); addr == nil {
		panic(fmt.Sprintf("the %s module account has not been set", types.RewardsPoolName))
	}

	if mintKeeper == nil {
		panic("mint keeper is nil")
	}

	if stakingKeeper == nil {
		panic("staking keeper is nil")
	}

	if accountKeeper == nil {
		panic("account keeper is nil")
	}

	if bankKeeper == nil {
		panic("bank keeper is nil")
	}

	if distributionKeeper == nil {
		panic("distribution keeper is nil")
	}

	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		cdc:                     cdc,
		storeService:            storeService,
		authority:               authority,
		mintKeeper:              mintKeeper,
		stakingKeeper:           stakingKeeper,
		accountKeeper:           accountKeeper,
		bankKeeper:              bankKeeper,
		distributionKeeper:      distributionKeeper,
		Params:                  collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Tiers:                   collections.NewMap(sb, types.TiersKey, "tiers", collections.Uint32Key, codec.CollValue[types.Tier](cdc)),
		Positions:               collections.NewMap(sb, types.PositionsKey, "positions", collections.Uint64Key, codec.CollValue[types.Position](cdc)),
		NextPositionId:          collections.NewSequence(sb, types.NextPositionIdKey, "next_position_id"),
		PositionsByOwner:        collections.NewKeySet(sb, types.PositionsByOwnerKey, "positions_by_owner", collections.PairKeyCodec(sdk.AccAddressKey, collections.Uint64Key)),
		PositionsByTier:         collections.NewKeySet(sb, types.PositionsByTierKey, "positions_by_tier", collections.PairKeyCodec(collections.Uint32Key, collections.Uint64Key)),
		PositionsByValidator:    collections.NewKeySet(sb, types.PositionsByValidatorKey, "positions_by_validator", collections.PairKeyCodec(sdk.ValAddressKey, collections.Uint64Key)),
		PositionCountByTier:     collections.NewMap(sb, types.PositionCountByTierKey, "position_count_by_tier", collections.Uint32Key, collections.Uint64Value),
		ValidatorRewardRatio:    collections.NewMap(sb, types.ValidatorRewardRatioKey, "validator_reward_ratio", sdk.ValAddressKey, codec.CollValue[types.ValidatorRewardRatio](cdc)),
		UnbondingIdToPositionId: collections.NewMap(sb, types.UnbondingIdToPositionIdKey, "unbonding_id_to_position_id", collections.Uint64Key, collections.Uint64Value),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the x/tieredrewards module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// SetParams validates and stores the module parameters.
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	return k.Params.Set(ctx, params)
}
