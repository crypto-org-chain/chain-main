> Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
> Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)

# Messages

## MsgIssueDenom

This message defines a type of non-fungible tokens, there can be multiple non-fungible tokens of the same type. Note
that both, `Id` and `Name`, are required to be unique globally.

| **Field** | **Type** | **Description**                                                                                                 |
| :-------- | :------- | :-------------------------------------------------------------------------------------------------------------- |
| Id        | `string` | The denomination ID of the NFT, necessary as multiple denominations are able to be represented on each chain.   |
| Name      | `string` | The denomination name of the NFT, necessary as multiple denominations are able to be represented on each chain. |
| Sender    | `string` | The account address of the user creating the denomination.                                                      |
| Schema    | `string` | NFT specifications defined under this category                                                                  |

```go
type MsgIssueDenom struct {
    Id     string
    Name   string
    Schema string
    Sender string
}
```

## MsgTransferNFT

This is the most commonly expected message type to be supported across chains. While each application specific
blockchain will have very different adoption of the `MsgMintNFT`, `MsgBurnNFT` and `MsgEditNFT` it should be expected
that most chains support the ability to transfer ownership of the non-fungible tokens. The exception to this would be
non-transferable NFTs that might be attached to reputation or some asset which should not be transferable. It still
makes sense for this to be represented as an NFT because there are common queriers which will remain relevant to the NFT
type even if non-transferable. `Sender` of this message should be the `Owner` of the NFT.

| **Field** | **Type** | **Description**                                                                                                  |
| :-------- | :------- | :--------------------------------------------------------------------------------------------------------------- |
| Id        | `string` | The unique ID of the NFT being transferred.                                                                      |
| DenomId   | `string` | The unique ID of the denomination, necessary as multiple denominations are able to be represented on each chain. |
| Sender    | `string` | The account address of the user sending the NFT.                                                                 |
| Recipient | `string` | The account address who will receive the NFT as a result of the transfer transaction.                            |

```go
// MsgTransferNFT defines an SDK message for transferring an NFT to recipient.
type MsgTransferNFT struct {
    Id        string
    DenomId   string
    Sender    string
    Recipient string
}
```

## MsgEditNFT

This message type allows the `TokenURI` to be updated. `Sender` of this message should be the `Owner` of the NFT and
`Creator` of the denomination corresponding to `DenomId`.

| **Field** | **Type** | **Description**                                                                                                  |
| :-------- | :------- | :--------------------------------------------------------------------------------------------------------------- |
| Id        | `string` | The unique ID of the NFT being edited.                                                                           |
| DenomId   | `string` | The unique ID of the denomination, necessary as multiple denominations are able to be represented on each chain. |
| Name      | `string` | The name of the NFT being edited.                                                                                |
| URI       | `string` | The URI pointing to a JSON object that contains subsequent tokenData information off-chain                       |
| Data      | `string` | The data of the NFT                                                                                              |
| Sender    | `string` | The creator of the message                                                                                       |

```go
// MsgEditNFT defines an SDK message for editing a nft.
type MsgEditNFT struct {
    Id      string
    DenomId string
    Name    string
    URI     string
    Data    string
    Sender  string
}
```

## MsgMintNFT

This message type is used for minting new non-fungible tokens. If a new token is minted under a new `Denom`, a new
`Collection` will also be created, otherwise the token is added to the existing `Collection`. `Sender` of the new token
should be the `Creator` of `Denom`. If `Recipient` is a new account, a new `Owner` is created, otherwise the NFT `Id` is
added to existing `Owner`'s `IDCollection`.

| **Field** | **Type** | **Description**                                                                            |
| :-------- | :------- | :----------------------------------------------------------------------------------------- |
| Id        | `string` | The unique ID of the NFT being minted                                                      |
| DenomId   | `string` | The unique ID of the denomination.                                                         |
| Name      | `string` | The name of the NFT being minted.                                                          |
| URI       | `string` | The URI pointing to a JSON object that contains subsequent tokenData information off-chain |
| Data      | `string` | The data of the NFT.                                                                       |
| Sender    | `string` | The sender of the Message                                                                  |
| Recipient | `string` | The recipient of the new NFT                                                                |

```go
// MsgMintNFT defines an SDK message for creating a new NFT.
type MsgMintNFT struct {
    Id        string
    DenomId   string
    Name      string
    URI       string
    Data      string
    Sender    string
    Recipient string
}
```

### MsgBurnNFT

This message type is used for burning non-fungible tokens which destroys and deletes them. `Sender` of this message
should be the `Owner` of the NFT and `Creator` of the denomination corresponding to `DenomId`.

| **Field** | **Type** | **Description**                                    |
| :-------- | :------- | :------------------------------------------------- |
| Id        | `string` | The ID of the Token.                               |
| DenomId   | `string` | The Denom ID of the Token.                         |
| Sender    | `string` | The account address of the user burning the token. |

```go
// MsgBurnNFT defines an SDK message for burning a NFT.
type MsgBurnNFT struct {
    Id      string
    DenomId string
    Sender  string
}
```
