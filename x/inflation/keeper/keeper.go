package keeper

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Keeper struct {
	cdc           codec.BinaryCodec
	storeService  store.KVStoreService
	logger        log.Logger
	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper

	// the address capable of executing a MsgUpdateParams message. Typically, this
	// should be the x/inflation module account.
	authority string

	cache *decayCache
}

type decayCache struct {
	DecayRate     math.LegacyDec
	BlocksPerYear uint64
	BlockFactor   math.LegacyDec
}

// NewKeeper creates a new inflation Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		bankKeeper:    bankKeeper,
		stakingKeeper: stakingKeeper,
		authority:     authority,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.logger.With("module", "x/"+types.ModuleName, "height", sdkCtx.BlockHeight())
}

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx context.Context) (params types.Params, err error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get([]byte(types.ParamsKey))
	if err != nil {
		return types.Params{}, err
	}
	if bz == nil {
		return types.Params{}, errors.New("params not found in store")
	}

	k.cdc.MustUnmarshal(bz, &params)
	return params, nil
}

// SetParams set the params
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	return store.Set([]byte(types.ParamsKey), bz)
}

// SetDecayEpochStart persists the block height at which inflation decay begins (e.g. upgrade activation height).
func (k Keeper) SetDecayEpochStart(ctx context.Context, height uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, height)
	return store.Set([]byte(types.DecayEpochStartKey), bz)
}

// getDecayEpochStart returns the block height at which inflation decay begins (e.g. upgrade activation height).
func (k Keeper) getDecayEpochStart(ctx context.Context) (uint64, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get([]byte(types.DecayEpochStartKey))
	if err != nil {
		return 0, false, err
	}
	if len(bz) == 0 {
		return 0, false, nil
	}
	if len(bz) != 8 {
		return 0, false, fmt.Errorf("invalid decay epoch start encoding: len=%d", len(bz))
	}
	return binary.BigEndian.Uint64(bz), true, nil
}

// GetAddressBalance returns the balance of the given address in the specified denomination.
func (k Keeper) GetAddressBalance(ctx context.Context, address, denom string) (math.Int, error) {
	addr, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return math.ZeroInt(), err
	}
	return k.bankKeeper.GetBalance(ctx, addr, denom).Amount, nil
}

// GetSupplyAndDenom returns the total supply and the bond denomination.
func (k Keeper) GetSupplyAndDenom(ctx context.Context) (math.Int, string, error) {
	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return math.ZeroInt(), "", err
	}
	return k.bankKeeper.GetSupply(ctx, bondDenom).Amount, bondDenom, nil
}

// GetAuthority returns the inflation module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

func (k *Keeper) GetDecayCache() *decayCache {
	if k.cache == nil {
		return nil
	}
	c := *k.cache
	return &c
}

func (k *Keeper) SetDecayCache(cache *decayCache) {
	k.cache = cache
}
