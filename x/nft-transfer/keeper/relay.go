package keeper

import (
	sdkerrors "cosmossdk.io/errors"
	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v5/modules/core/24-host"
	coretypes "github.com/cosmos/ibc-go/v5/modules/core/types"
	"github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

// SendTransfer handles nft-transfer sending logic.
// A sending chain may be acting as a source or sink zone.
//
// when a chain is sending tokens across a port and channel which are
// not equal to the last prefixed port and channel pair, it is acting as a source zone.
// when tokens are sent from a source zone, the destination port and
// channel will be prefixed onto the classId (once the tokens are received)
// adding another hop to the tokens record.
//
// when a chain is sending tokens across a port and channel which are
// equal to the last prefixed port and channel pair, it is acting as a sink zone.
// when tokens are sent from a sink zone, the last prefixed port and channel
// pair on the classId is removed (once the tokens are received), undoing the last hop in the tokens record.
//
// For example, assume these steps of transfer occur:
// A -> B -> C -> A -> C -> B -> A
//
// |                    sender  chain                      |                       receiver     chain              |
// | :-----: | -------------------------: | :------------: | :------------: | -------------------------: | :-----: |
// |  chain  |                    classID | (port,channel) | (port,channel) |                    classID |  chain  |
// |    A    |                   nftClass |    (p1,c1)     |    (p2,c2)     |             p2/c2/nftClass |    B    |
// |    B    |             p2/c2/nftClass |    (p3,c3)     |    (p4,c4)     |       p4/c4/p2/c2/nftClass |    C    |
// |    C    |       p4/c4/p2/c2/nftClass |    (p5,c5)     |    (p6,c6)     | p6/c6/p4/c4/p2/c2/nftClass |    A    |
// |    A    | p6/c6/p4/c4/p2/c2/nftClass |    (p6,c6)     |    (p5,c5)     |       p4/c4/p2/c2/nftClass |    C    |
// |    C    |       p4/c4/p2/c2/nftClass |    (p4,c4)     |    (p3,c3)     |             p2/c2/nftClass |    B    |
// |    B    |             p2/c2/nftClass |    (p2,c2)     |    (p1,c1)     |                   nftClass |    A    |
func (k Keeper) SendTransfer(
	ctx sdk.Context,
	sourcePort,
	sourceChannel,
	classID string,
	tokenIDs []string,
	sender sdk.AccAddress,
	receiver string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) error {
	sourceChannelEnd, found := k.channelKeeper.GetChannel(ctx, sourcePort, sourceChannel)
	if !found {
		return sdkerrors.Wrapf(channeltypes.ErrChannelNotFound, "port ID (%s) channel ID (%s)", sourcePort, sourceChannel)
	}

	destinationPort := sourceChannelEnd.GetCounterparty().GetPortID()
	destinationChannel := sourceChannelEnd.GetCounterparty().GetChannelID()

	// get the next sequence
	sequence, found := k.channelKeeper.GetNextSequenceSend(ctx, sourcePort, sourceChannel)
	if !found {
		return sdkerrors.Wrapf(
			channeltypes.ErrSequenceSendNotFound,
			"source port: %s, source channel: %s", sourcePort, sourceChannel,
		)
	}

	channelCap, ok := k.scopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(sourcePort, sourceChannel))
	if !ok {
		return sdkerrors.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}

	// See spec for this logic: https://github.com/cosmos/ibc/blob/master/spec/app/ics-721-nft-transfer/README.md#packet-relay
	packet, err := k.createOutgoingPacket(ctx,
		sourcePort,
		sourceChannel,
		destinationPort,
		destinationChannel,
		classID,
		tokenIDs,
		sender,
		receiver,
		sequence,
		timeoutHeight,
		timeoutTimestamp,
	)
	if err != nil {
		return err
	}

	if err := k.ics4Wrapper.SendPacket(ctx, channelCap, packet); err != nil {
		return err
	}

	defer func() {
		labels := []metrics.Label{
			telemetry.NewLabel(coretypes.LabelDestinationPort, destinationPort),
			telemetry.NewLabel(coretypes.LabelDestinationChannel, destinationChannel),
		}

		telemetry.SetGaugeWithLabels(
			[]string{"tx", "msg", "ibc", "nft-transfer"},
			float32(len(tokenIDs)),
			[]metrics.Label{telemetry.NewLabel("class_id", classID)},
		)

		telemetry.IncrCounterWithLabels(
			[]string{"ibc", types.ModuleName, "send"},
			1,
			labels,
		)
	}()
	return nil
}

// OnRecvPacket processes a cross chain fungible token transfer. If the
// sender chain is the source of minted tokens then vouchers will be minted
// and sent to the receiving address. Otherwise if the sender chain is sending
// back tokens this chain originally transferred to it, the tokens are
// unescrowed and sent to the receiving address.
func (k Keeper) OnRecvPacket(ctx sdk.Context, packet channeltypes.Packet,
	data types.NonFungibleTokenPacketData,
) error {
	// validate packet data upon receiving
	if err := data.ValidateBasic(); err != nil {
		return err
	}

	// See spec for this logic: https://github.com/cosmos/ibc/blob/master/spec/app/ics-721-nft-transfer/README.md#packet-relay
	return k.processReceivedPacket(ctx, packet, data)
}

// OnAcknowledgementPacket responds to the the success or failure of a packet
// acknowledgement written on the receiving chain. If the acknowledgement
// was a success then nothing occurs. If the acknowledgement failed, then
// the sender is refunded their tokens using the refundPacketToken function.
func (k Keeper) OnAcknowledgementPacket(ctx sdk.Context, packet channeltypes.Packet, data types.NonFungibleTokenPacketData, ack channeltypes.Acknowledgement) error {
	switch ack.Response.(type) {
	case *channeltypes.Acknowledgement_Error:
		return k.refundPacketToken(ctx, packet, data)
	default:
		// the acknowledgement succeeded on the receiving chain so nothing
		// needs to be executed and no error needs to be returned
		return nil
	}
}

// OnTimeoutPacket refunds the sender since the original packet sent was
// never received and has been timed out.
func (k Keeper) OnTimeoutPacket(ctx sdk.Context, packet channeltypes.Packet, data types.NonFungibleTokenPacketData) error {
	return k.refundPacketToken(ctx, packet, data)
}
