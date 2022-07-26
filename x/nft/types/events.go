// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package types

// NFT module event types
var (
	EventTypeIssueDenom = "issue_denom"
	EventTypeTransfer   = "transfer_nft"
	EventTypeEditNFT    = "edit_nft"
	EventTypeMintNFT    = "mint_nft"
	EventTypeBurnNFT    = "burn_nft"

	AttributeValueCategory = ModuleName

	AttributeKeySender    = "sender"
	AttributeKeyCreator   = "creator"
	AttributeKeyRecipient = "recipient"
	AttributeKeyOwner     = "owner"
	AttributeKeyTokenID   = "token_id"
	AttributeKeyTokenURI  = "token_uri"
	AttributeKeyDenomID   = "denom_id"
	AttributeKeyDenomName = "denom_name"
)
