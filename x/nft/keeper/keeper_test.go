// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package keeper_test

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/chain-main/v4/x/nft/keeper"
	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

var (
	denomID = "denomid"
	denomNm = "denomnm"
	schema  = "{a:a,b:b}"

	denomID2 = "denomid2"
	denomNm2 = "denom2nm"

	tokenID  = "tokenid"
	tokenID2 = "tokenid2"
	tokenID3 = "tokenid3"

	tokenNm  = "tokennm"
	tokenNm2 = "tokennm2"
	tokenNm3 = "tokennm3"

	address   = CreateTestAddrs(1)[0]
	address2  = CreateTestAddrs(2)[1]
	address3  = CreateTestAddrs(3)[2]
	tokenURI  = "https://google.com/token-1.json" // nolint: gosec
	tokenURI2 = "https://google.com/token-2.json" // nolint: gosec
	tokenData = "{a:a,b:b}"                       // nolint: gosec

	isCheckTx = false
)

// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
type KeeperSuite struct {
	suite.Suite

	legacyAmino *codec.LegacyAmino
	ctx         sdk.Context
	keeper      keeper.Keeper
	app         *app.ChainApp

	queryClient types.QueryClient
}

func (suite *KeeperSuite) SetupTest() {

	app := app.Setup(suite.T(), isCheckTx)

	suite.app = app
	suite.legacyAmino = app.LegacyAmino()
	suite.ctx = app.BaseApp.NewContext(isCheckTx, tmproto.Header{})
	suite.keeper = app.NFTKeeper

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, app.NFTKeeper)
	suite.queryClient = types.NewQueryClient(queryHelper)

	err := suite.keeper.IssueDenom(suite.ctx, denomID, denomNm, schema, "", address)
	suite.NoError(err)

	// MintNFT shouldn't fail when collection does not exist
	err = suite.keeper.IssueDenom(suite.ctx, denomID2, denomNm2, schema, "", address)
	suite.NoError(err)

	// collections should equal 1
	collections := suite.keeper.GetCollections(suite.ctx)
	suite.NotEmpty(collections)
	suite.Equal(len(collections), 2)
}

func TestKeeperSuite(t *testing.T) {
	suite.Run(t, new(KeeperSuite))
}

func (suite *KeeperSuite) TestMintNFT() {
	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	// MintNFT shouldn't fail when collection exists
	err = suite.keeper.MintNFT(suite.ctx, denomID, tokenID2, tokenNm2, tokenURI, tokenData, address, address)
	suite.NoError(err)

	// MintNFT should fail when owner address is not the creator of denom
	err = suite.keeper.MintNFT(suite.ctx, denomID, tokenID3, tokenNm3, tokenURI, tokenData, address2, address2)
	suite.Error(err)
}

func (suite *KeeperSuite) TestUpdateNFT() {
	// EditNFT should fail when NFT doesn't exists
	err := suite.keeper.EditNFT(suite.ctx, denomID, tokenID, tokenNm3, tokenURI, tokenData, address)
	suite.Error(err)

	// MintNFT shouldn't fail when collection does not exist
	err = suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	// EditNFT should fail when NFT doesn't exists
	err = suite.keeper.EditNFT(suite.ctx, denomID, tokenID2, tokenNm2, tokenURI, tokenData, address)
	suite.Error(err)

	// EditNFT shouldn't fail when NFT exists
	err = suite.keeper.EditNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI2, tokenData, address)
	suite.NoError(err)

	// GetNFT should get the NFT with new tokenURI
	receivedNFT, err := suite.keeper.GetNFT(suite.ctx, denomID, tokenID)
	suite.NoError(err)
	suite.Equal(receivedNFT.GetURI(), tokenURI2)

	// EditNFT should fail when address is not the owner
	err = suite.keeper.EditNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address2)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("%s is not the owner of %s/%s: %s", address2, denomID, tokenID, types.ErrUnauthorized))
	}

	// Transfer owner shouldn't fail
	err = suite.keeper.TransferOwner(suite.ctx, denomID, tokenID, address, address2)
	suite.NoError(err)

	// EditNFT should fail when address is not the creator of denom
	err = suite.keeper.EditNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address2)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("%s is not the creator of %s: %s", address2, denomID, types.ErrUnauthorized))
	}
}

func (suite *KeeperSuite) TestTransferOwner() {

	// MintNFT shouldn't fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	// invalid owner
	err = suite.keeper.TransferOwner(suite.ctx, denomID, tokenID, address2, address3)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("%s is not the owner of %s/%s: %s", address2, denomID, tokenID, types.ErrUnauthorized))
	}

	// right
	err = suite.keeper.TransferOwner(suite.ctx, denomID, tokenID, address, address2)
	suite.NoError(err)

	nft, err := suite.keeper.GetNFT(suite.ctx, denomID, tokenID)
	suite.NoError(err)
	suite.Equal(tokenURI, nft.GetURI())
}

func (suite *KeeperSuite) TestBurnNFT() {
	// MintNFT should not fail when collection does not exist
	err := suite.keeper.MintNFT(suite.ctx, denomID, tokenID, tokenNm, tokenURI, tokenData, address, address)
	suite.NoError(err)

	// BurnNFT should fail when address is not the owner of nft
	err = suite.keeper.BurnNFT(suite.ctx, denomID, tokenID, address2)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("%s is not the owner of %s/%s: %s", address2, denomID, tokenID, types.ErrUnauthorized))
	}

	// Transfer nft to an address which is not the creator of denom
	err = suite.keeper.TransferOwner(suite.ctx, denomID, tokenID, address, address2)
	suite.NoError(err)

	// BurnNFT should fail when address is not the creator of denom
	err = suite.keeper.BurnNFT(suite.ctx, denomID, tokenID, address2)
	if suite.Error(err) {
		suite.EqualError(err, fmt.Sprintf("%s is not the creator of %s: %s", address2, denomID, types.ErrUnauthorized))
	}

	// Transfer nft back to the creator of denom
	err = suite.keeper.TransferOwner(suite.ctx, denomID, tokenID, address2, address)
	suite.NoError(err)

	// BurnNFT shouldn't fail when NFT exists and address is owner of nft and creator of denom
	err = suite.keeper.BurnNFT(suite.ctx, denomID, tokenID, address)
	suite.NoError(err)

	// NFT should no longer exist
	isNFT := suite.keeper.HasNFT(suite.ctx, denomID, tokenID)
	suite.False(isNFT)

	msg, fail := keeper.SupplyInvariant(suite.keeper)(suite.ctx)
	suite.False(fail, msg)
}

// CreateTestAddrs creates test addresses
func CreateTestAddrs(numAddrs int) []sdk.AccAddress {
	var addresses []sdk.AccAddress // nolint: prealloc
	var buffer bytes.Buffer

	// start at 100 so we can make up to 999 test addresses with valid test addresses
	for i := 100; i < (numAddrs + 100); i++ {
		numString := strconv.Itoa(i)
		buffer.WriteString("A58856F0FD53BF058B4909A21AEC019107BA6") // base address string

		buffer.WriteString(numString)                          // adding on final two digits to make addresses unique
		res, _ := sdk.AccAddressFromHexUnsafe(buffer.String()) // nolint: errcheck
		bech := res.String()
		addresses = append(addresses, testAddr(buffer.String(), bech))
		buffer.Reset()
	}

	return addresses
}

// for incode address generation
func testAddr(addr string, bech string) sdk.AccAddress {
	res, err := sdk.AccAddressFromHexUnsafe(addr)
	if err != nil {
		panic(err)
	}
	bechexpected := res.String()
	if bech != bechexpected {
		panic("Bech encoding doesn't match reference")
	}

	bechres, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(bechres, res) {
		panic("Bech decode and hex decode don't match")
	}

	return res
}
