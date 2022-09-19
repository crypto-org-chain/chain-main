package types

import (
	"crypto/sha256"
	fmt "fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Module name defines IBC nft-transfer moduel name
	ModuleName = "nonfungibletokentransfer"

	// StoreKey is the store key string for IBC nft-transfer
	StoreKey = ModuleName

	// RouterKey is the message route for IBC nft-transfer
	RouterKey = ModuleName

	// QuerierRoute is the querier route for IBC nft-transfer
	QuerierRoute = ModuleName

	// Version defines the current version the IBC nft-transfer
	// module supports
	Version = "ics721-1"

	// PortID is the default port id that nft-transfer module binds to
	PortID = "nft"

	// ClassPrefix is the prefix used for internal SDK NFT representation.
	ClassPrefix = "ibc"
)

var (
	// PortKey defines the key to store the port ID in store
	PortKey = []byte{0x01}

	// ClassTraceKey defines the key to store the class trace info in store
	ClassTraceKey = []byte{0x02}
)

// GetEscrowAddress returns the escrow address for the specified channel.
// The escrow address follows the format as outlined in ADR 028:
// https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-028-public-key-addresses.md
func GetEscrowAddress(portID string, channelID string) sdk.AccAddress {
	// a slash is used to create domain separation between port and channel identifiers to
	// prevent address collisions between escrow addresses created for different channels
	contents := fmt.Sprintf("%s/%s", portID, channelID)

	// ADR 028 AddressHash construction
	preImage := []byte(Version)
	preImage = append(preImage, 0)
	preImage = append(preImage, contents...)
	hash := sha256.Sum256(preImage)
	return hash[:20]
}
