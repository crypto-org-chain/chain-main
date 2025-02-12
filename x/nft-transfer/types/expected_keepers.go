package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v9/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v9/modules/core/04-channel/types"
	nftexported "github.com/crypto-org-chain/chain-main/v4/x/nft/exported"
	nfttypes "github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

// ICS4Wrapper defines the expected ICS4Wrapper for middleware
type ICS4Wrapper interface {
	SendPacket(
		ctx context.Context,
		sourcePort string,
		sourceChannel string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
		data []byte,
	) (uint64, error)
}

// ChannelKeeper defines the expected IBC channel keeper
type ChannelKeeper interface {
	GetChannel(ctx context.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool)
	GetNextSequenceSend(ctx context.Context, portID, channelID string) (uint64, bool)
}

// NFTKeeper defines the expected nft keeper
type NFTKeeper interface {
	HasDenomID(ctx context.Context, id string) bool
	GetDenom(ctx context.Context, id string) (denom nfttypes.Denom, err error)
	IssueDenom(ctx context.Context, id, name, schema, uri string, creator sdk.AccAddress) error

	GetNFT(ctx context.Context, denomID, tokenID string) (nft nftexported.NFT, err error)
	MintNFT(
		ctx context.Context, denomID, tokenID, tokenNm,
		tokenURI, tokenData string, sender, owner sdk.AccAddress,
	) error
	BurnNFTUnverified(ctx context.Context, denomID, tokenID string, owner sdk.AccAddress) error
	TransferOwner(ctx context.Context, denomID, tokenID string, srcOwner, dstOwner sdk.AccAddress) error
}

// AccountKeeper defines the contract required for account APIs.
type AccountKeeper interface {
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	// Set an account in the store.
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	HasAccount(ctx context.Context, addr sdk.AccAddress) bool
	SetAccount(ctx context.Context, acc sdk.AccountI)
}
