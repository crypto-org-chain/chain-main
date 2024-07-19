package keeper

import (
	"fmt"
	"time"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/gogoproto/proto"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/keeper"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	Keeper struct {
		cdc        codec.Codec
		storeKey   storetypes.StoreKey
		memKey     storetypes.StoreKey
		paramStore paramtypes.Subspace

		icaControllerKeeper icacontrollerkeeper.Keeper
		scopedKeeper        capabilitykeeper.ScopedKeeper
	}
)

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey storetypes.StoreKey,
	paramStore paramtypes.Subspace,
	icaControllerKeeper icacontrollerkeeper.Keeper,
	scopedKeeper capabilitykeeper.ScopedKeeper,
) *Keeper {
	// set KeyTable if it has not already been set
	if !paramStore.HasKeyTable() {
		paramStore = paramStore.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:        cdc,
		storeKey:   storeKey,
		memKey:     memKey,
		paramStore: paramStore,

		icaControllerKeeper: icaControllerKeeper,
		scopedKeeper:        scopedKeeper,
	}
}

// DoSubmitTx submits a transaction to the host chain on behalf of interchain account
func (k *Keeper) DoSubmitTx(ctx sdk.Context, connectionID, owner string, msgs []sdk.Msg, timeoutDuration time.Duration) error {
	portID, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return err
	}

	protoMsgs := make([]proto.Message, len(msgs))
	for i, msg := range msgs {
		protoMsgs[i] = msg.(proto.Message)
	}
	data, err := icatypes.SerializeCosmosTx(k.cdc, protoMsgs, icatypes.EncodingProtobuf)
	if err != nil {
		return err
	}

	packetData := icatypes.InterchainAccountPacketData{
		Type: icatypes.EXECUTE_TX,
		Data: data,
	}

	timeoutTimestamp := ctx.BlockTime().Add(timeoutDuration).UnixNano()

	_, err = k.icaControllerKeeper.SendTx(ctx, nil, connectionID, portID, packetData, uint64(timeoutTimestamp)) //nolint:staticcheck
	if err != nil {
		return err
	}

	return nil
}

// RegisterInterchainAccount registers an interchain account with the given `connectionId` and `owner` on the host chain
func (k *Keeper) RegisterInterchainAccount(ctx sdk.Context, connectionID, owner string, version string) error {
	return k.icaControllerKeeper.RegisterInterchainAccount(ctx, connectionID, owner, version)
}

// GetInterchainAccountAddress fetches the interchain account address for given `connectionId` and `owner`
func (k *Keeper) GetInterchainAccountAddress(ctx sdk.Context, connectionID, owner string) (string, error) {
	portID, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid owner address: %s", err)
	}

	icaAddress, found := k.icaControllerKeeper.GetInterchainAccountAddress(ctx, connectionID, portID)

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
