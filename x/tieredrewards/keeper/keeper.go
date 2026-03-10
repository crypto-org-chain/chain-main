package keeper

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Keeper of the tieredrewards store.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService storetypes.KVStoreService
	authority    string

	Schema collections.Schema
	Params collections.Item[types.Params]

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
		Params:             collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
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

func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	return k.Params.Get(ctx)
}

func (k Keeper) GetBlocksPerYear(ctx context.Context) (uint64, error) {
	params, err := k.mintKeeper.GetParams(ctx)
	if err != nil {
		return 0, err
	}
	return params.BlocksPerYear, nil
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
