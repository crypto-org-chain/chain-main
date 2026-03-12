package keeper

import (
	"context"
	"errors"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/indexes"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// PositionIndexes defines the secondary indexes for TierPosition.
type PositionIndexes struct {
	Owner     *indexes.Multi[string, uint64, types.TierPosition]
	Validator *indexes.Multi[string, uint64, types.TierPosition]
}

func (i PositionIndexes) IndexesList() []collections.Index[uint64, types.TierPosition] {
	return []collections.Index[uint64, types.TierPosition]{i.Owner, i.Validator}
}

func newPositionIndexes(sb *collections.SchemaBuilder) PositionIndexes {
	return PositionIndexes{
		Owner: indexes.NewMulti(sb, types.PositionsByOwnerPrefix, "positions_by_owner",
			collections.StringKey, collections.Uint64Key,
			func(_ uint64, v types.TierPosition) (string, error) {
				return v.Owner, nil
			}),
		Validator: indexes.NewMulti(sb, types.PositionsByValidatorPrefix, "positions_by_validator",
			collections.StringKey, collections.Uint64Key,
			func(_ uint64, v types.TierPosition) (string, error) {
				return v.Validator, nil
			}),
	}
}

// Keeper of the tieredrewards store.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService storetypes.KVStoreService
	authority    string

	Schema collections.Schema
	Params collections.Item[types.Params]

	// Positions stores all tier positions keyed by position ID, with secondary index on Owner.
	Positions *collections.IndexedMap[uint64, types.TierPosition, PositionIndexes]
	// NextPositionID is a sequence that produces monotonically increasing position IDs.
	NextPositionID collections.Sequence
	// TotalTierShares tracks the total delegated shares per validator across all tier positions.
	TotalTierShares collections.Map[string, math.LegacyDec]
	// UnbondingPositions tracks position IDs that are currently unbonding.
	// Key = position_id, Value = unbonding completion time as unix seconds.
	UnbondingPositions collections.Map[uint64, int64]

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

	// ensure tier reward pool module account is set
	if addr := accountKeeper.GetModuleAddress(types.TierPoolName); addr == nil {
		panic(fmt.Sprintf("the %s module account has not been set", types.TierPoolName))
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
		cdc:                cdc,
		storeService:       storeService,
		authority:          authority,
		mintKeeper:         mintKeeper,
		stakingKeeper:      stakingKeeper,
		accountKeeper:      accountKeeper,
		bankKeeper:         bankKeeper,
		distributionKeeper: distributionKeeper,
		Params:          collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Positions:        collections.NewIndexedMap(sb, types.PositionByIDPrefix, "positions", collections.Uint64Key, codec.CollValue[types.TierPosition](cdc), newPositionIndexes(sb)),
		NextPositionID:   collections.NewSequence(sb, types.NextPositionIDKey, "next_position_id"),
		TotalTierShares:  collections.NewMap(sb, types.TotalTierSharesPrefix, "total_tier_shares", collections.StringKey, sdk.LegacyDecValue),
		UnbondingPositions: collections.NewMap(sb, types.UnbondingPositionsPrefix, "unbonding_positions", collections.Uint64Key, collections.Int64Value),
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

func (k Keeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	return k.stakingKeeper.TotalBondedTokens(ctx)
}

func (k Keeper) BondDenom(ctx context.Context) (string, error) {
	return k.stakingKeeper.BondDenom(ctx)
}

func (k Keeper) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	return k.accountKeeper.GetModuleAccount(ctx, moduleName)
}

func (k Keeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return k.accountKeeper.GetModuleAddress(moduleName)
}

func (k Keeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return k.bankKeeper.GetBalance(ctx, addr, denom)
}

func (k Keeper) SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	return k.bankKeeper.SendCoinsFromModuleToModule(ctx, senderModule, recipientModule, amt)
}

func (k Keeper) GetCommunityTax(ctx context.Context) (math.LegacyDec, error) {
	return k.distributionKeeper.GetCommunityTax(ctx)
}

func (k Keeper) AllocateTokensToValidator(ctx context.Context, val stakingtypes.ValidatorI, tokens sdk.DecCoins) error {
	return k.distributionKeeper.AllocateTokensToValidator(ctx, val, tokens)
}

func (k Keeper) ValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.ValidatorI, error) {
	return k.stakingKeeper.ValidatorByConsAddr(ctx, consAddr)
}

// AddTierShares adds shares to the running total for a validator.
func (k Keeper) AddTierShares(ctx context.Context, validator string, shares math.LegacyDec) error {
	if shares.IsZero() {
		return nil
	}
	current, err := k.GetTotalTierShares(ctx, validator)
	if err != nil {
		return err
	}
	return k.TotalTierShares.Set(ctx, validator, current.Add(shares))
}

// SubTierShares subtracts shares from the running total for a validator.
func (k Keeper) SubTierShares(ctx context.Context, validator string, shares math.LegacyDec) error {
	if shares.IsZero() {
		return nil
	}
	current, err := k.GetTotalTierShares(ctx, validator)
	if err != nil {
		return err
	}
	newTotal := current.Sub(shares)
	if newTotal.IsNegative() {
		return fmt.Errorf("TotalTierShares for validator %s would go negative: current=%s, sub=%s", validator, current, shares)
	}
	if newTotal.IsZero() {
		return k.TotalTierShares.Remove(ctx, validator)
	}
	return k.TotalTierShares.Set(ctx, validator, newTotal)
}

// GetTotalTierShares returns the total tier shares for a validator. Returns zero if not found.
func (k Keeper) GetTotalTierShares(ctx context.Context, validator string) (math.LegacyDec, error) {
	total, err := k.TotalTierShares.Get(ctx, validator)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return math.LegacyZeroDec(), nil
		}
		return math.LegacyZeroDec(), err
	}
	return total, nil
}

func (k Keeper) GetMintParams(ctx context.Context) (minttypes.Params, error) {
	p, err := k.mintKeeper.GetParams(ctx)
	if err != nil {
		return minttypes.Params{}, err
	}
	return p, nil
}
