package keeper

import (
	"context"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	host "github.com/cosmos/ibc-go/v9/modules/core/24-host"
	"github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

// Keeper defines the IBC non fungible transfer keeper
type Keeper struct {
	storeKey storetypes.StoreKey
	cdc      codec.BinaryCodec

	ics4Wrapper   types.ICS4Wrapper
	channelKeeper types.ChannelKeeper
	nftKeeper     types.NFTKeeper
	authKeeper    types.AccountKeeper
}

// NewKeeper creates a new IBC nft-transfer Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	key storetypes.StoreKey,
	ics4Wrapper types.ICS4Wrapper,
	channelKeeper types.ChannelKeeper,
	nftKeeper types.NFTKeeper,
	authKeeper types.AccountKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeKey:      key,
		ics4Wrapper:   ics4Wrapper,
		channelKeeper: channelKeeper,
		nftKeeper:     nftKeeper,
		authKeeper:    authKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+host.SubModuleName+"-"+types.ModuleName)
}

// SetPort sets the portID for the nft-transfer module. Used in InitGenesis
func (k Keeper) SetPort(ctx context.Context, portID string) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Set(types.PortKey, []byte(portID))
}

// GetPort returns the portID for the nft-transfer module.
func (k Keeper) GetPort(ctx context.Context) string {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	return string(store.Get(types.PortKey))
}

// SetEscrowAddress attempts to save a account to auth module
func (k Keeper) SetEscrowAddress(ctx context.Context, portID, channelID string) {
	// create the escrow address for the tokens
	escrowAddress := types.GetEscrowAddress(portID, channelID)
	if !k.authKeeper.HasAccount(ctx, escrowAddress) {
		acc := k.authKeeper.NewAccountWithAddress(ctx, escrowAddress)
		k.authKeeper.SetAccount(ctx, acc)
	}
}
