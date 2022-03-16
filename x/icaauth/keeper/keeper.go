package keeper

import (
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/keeper"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"github.com/tendermint/tendermint/libs/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	Keeper struct {
		cdc      codec.BinaryCodec
		storeKey sdk.StoreKey
		memKey   sdk.StoreKey

		icaControllerKeeper icacontrollerkeeper.Keeper
		scopedKeeper        capabilitykeeper.ScopedKeeper
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey,
	memKey sdk.StoreKey,
	icaControllerKeeper icacontrollerkeeper.Keeper,
	scopedKeeper capabilitykeeper.ScopedKeeper,
) *Keeper {
	return &Keeper{
		cdc:      cdc,
		storeKey: storeKey,
		memKey:   memKey,

		icaControllerKeeper: icaControllerKeeper,
		scopedKeeper:        scopedKeeper,
	}
}

// DoSubmitTx submits a transaction to the host chain on behalf of interchain account
func (k *Keeper) DoSubmitTx(ctx sdk.Context, connectionId, owner string, msgs []sdk.Msg, timeoutDuration time.Duration) error {
	portId, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return err
	}

	channelId, found := k.icaControllerKeeper.GetActiveChannelID(ctx, connectionId, portId)
	if !found {
		return sdkerrors.Wrapf(icatypes.ErrActiveChannelNotFound, "failed to retrieve active channel for port %s", portId)
	}

	channelCapability, found := k.scopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(portId, channelId))
	if !found {
		return sdkerrors.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}

	data, err := icatypes.SerializeCosmosTx(k.cdc, msgs)
	if err != nil {
		return err
	}

	packetData := icatypes.InterchainAccountPacketData{
		Type: icatypes.EXECUTE_TX,
		Data: data,
	}

	timeoutTimestamp := ctx.BlockTime().Add(timeoutDuration).UnixNano()

	_, err = k.icaControllerKeeper.SendTx(ctx, channelCapability, connectionId, portId, packetData, uint64(timeoutTimestamp))
	if err != nil {
		return err
	}

	return nil
}

// GetInterchainAccountAddress fetches the interchain account address for given `connectionId` and `owner`
func (k *Keeper) GetInterchainAccountAddress(ctx sdk.Context, connectionId, owner string) (string, error) {
	portId, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid owner address: %s", err)
	}

	icaAddress, found := k.icaControllerKeeper.GetInterchainAccountAddress(ctx, connectionId, portId)

	if !found {
		return "", status.Errorf(codes.NotFound, "could not find account")
	}

	return icaAddress, nil
}

// ClaimCapability claims the channel capability passed via the OnOpenChanInit callback
func (k *Keeper) ClaimCapability(ctx sdk.Context, cap *capabilitytypes.Capability, name string) error {
	return k.scopedKeeper.ClaimCapability(ctx, cap, name)
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
