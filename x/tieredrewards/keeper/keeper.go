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

	Positions      collections.Map[uint64, types.Position]
	NextPositionId collections.Sequence

	PositionsByOwner collections.KeySet[collections.Pair[sdk.AccAddress, uint64]]
	PositionsByTier  collections.KeySet[collections.Pair[uint32, uint64]]

	PositionCountByTier      collections.Map[uint32, uint64]
	PositionCountByValidator collections.Map[sdk.ValAddress, uint64]

	// Validator events for lazy processing: (valAddr, seq) -> ValidatorEvent.
	ValidatorEvents   collections.Map[collections.Pair[sdk.ValAddress, uint64], types.ValidatorEvent]
	ValidatorEventSeq collections.Map[sdk.ValAddress, uint64]

	// RedelegationMappings maps a redelegation unbonding id to the tier position id that issued the redelegation.
	RedelegationMappings *collections.IndexedMap[uint64, uint64, RedelegationMappingsIndexes]

	mintKeeper         types.MintKeeper
	stakingKeeper      types.StakingKeeper
	accountKeeper      types.AccountKeeper
	bankKeeper         types.BankKeeper
	distributionKeeper types.DistributionKeeper
}

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
		cdc:                      cdc,
		storeService:             storeService,
		authority:                authority,
		mintKeeper:               mintKeeper,
		stakingKeeper:            stakingKeeper,
		accountKeeper:            accountKeeper,
		bankKeeper:               bankKeeper,
		distributionKeeper:       distributionKeeper,
		Params:                   collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Tiers:                    collections.NewMap(sb, types.TiersKey, "tiers", collections.Uint32Key, codec.CollValue[types.Tier](cdc)),
		Positions:                collections.NewMap(sb, types.PositionsKey, "positions", collections.Uint64Key, codec.CollValue[types.Position](cdc)),
		NextPositionId:           collections.NewSequence(sb, types.NextPositionIdKey, "next_position_id"),
		PositionsByOwner:         collections.NewKeySet(sb, types.PositionsByOwnerKey, "positions_by_owner", collections.PairKeyCodec(sdk.AccAddressKey, collections.Uint64Key)),
		PositionsByTier:          collections.NewKeySet(sb, types.PositionsByTierKey, "positions_by_tier", collections.PairKeyCodec(collections.Uint32Key, collections.Uint64Key)),
		PositionCountByTier:      collections.NewMap(sb, types.PositionCountByTierKey, "position_count_by_tier", collections.Uint32Key, collections.Uint64Value),
		PositionCountByValidator: collections.NewMap(sb, types.PositionCountByValidatorKey, "position_count_by_validator", sdk.ValAddressKey, collections.Uint64Value),
		ValidatorEvents:          collections.NewMap(sb, types.ValidatorEventsKey, "validator_events", collections.PairKeyCodec(sdk.ValAddressKey, collections.Uint64Key), codec.CollValue[types.ValidatorEvent](cdc)),
		ValidatorEventSeq:        collections.NewMap(sb, types.ValidatorEventSeqKey, "validator_event_current_seq", sdk.ValAddressKey, collections.Uint64Value),
		RedelegationMappings:     collections.NewIndexedMap(sb, types.RedelegationMappingsKey, "redelegation_mappings", collections.Uint64Key, collections.Uint64Value, newRedelegationMappingsIndexes(sb)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

func (k Keeper) getAuthority() string {
	return k.authority
}

func (k Keeper) logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	return k.Params.Set(ctx, params)
}
