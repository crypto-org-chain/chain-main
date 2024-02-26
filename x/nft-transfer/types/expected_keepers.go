package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	nftexported "github.com/crypto-org-chain/chain-main/v4/x/nft/exported"
	nfttypes "github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

// ICS4Wrapper defines the expected ICS4Wrapper for middleware
type ICS4Wrapper interface {
	SendPacket(
		ctx sdk.Context,
		chanCap *capabilitytypes.Capability,
		sourcePort string,
		sourceChannel string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
		data []byte,
	) (uint64, error)
}

// ChannelKeeper defines the expected IBC channel keeper
type ChannelKeeper interface {
	GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool)
	GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool)
}

// NFTKeeper defines the expected nft keeper
type NFTKeeper interface {
	HasDenomID(ctx sdk.Context, id string) bool
	GetDenom(ctx sdk.Context, id string) (denom nfttypes.Denom, err error)
	IssueDenom(ctx sdk.Context, id, name, schema, uri string, creator sdk.AccAddress) error

	GetNFT(ctx sdk.Context, denomID, tokenID string) (nft nftexported.NFT, err error)
	MintNFT(
		ctx sdk.Context, denomID, tokenID, tokenNm,
		tokenURI, tokenData string, sender, owner sdk.AccAddress,
	) error
	BurnNFTUnverified(ctx sdk.Context, denomID, tokenID string, owner sdk.AccAddress) error
	TransferOwner(ctx sdk.Context, denomID, tokenID string, srcOwner, dstOwner sdk.AccAddress) error
}

// PortKeeper defines the expected IBC port keeper
type PortKeeper interface {
	BindPort(ctx sdk.Context, portID string) *capabilitytypes.Capability
}

// AccountKeeper defines the contract required for account APIs.
type AccountKeeper interface {
	NewAccountWithAddress(ctx sdk.Context, addr sdk.AccAddress) types.AccountI
	// Set an account in the store.
	SetAccount(sdk.Context, types.AccountI)
	HasAccount(ctx sdk.Context, addr sdk.AccAddress) bool
	GetModuleAddress(name string) sdk.AccAddress
}
