// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021, CRO Protocol Labs ("Crypto.org") (licensed under the Apache License, Version 2.0)
package rest_test

import (
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"
	"github.com/tidwall/gjson"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"

	"github.com/crypto-org-chain/chain-main/v2/app"
	nftcli "github.com/crypto-org-chain/chain-main/v2/x/nft/client/cli"
	nfttestutil "github.com/crypto-org-chain/chain-main/v2/x/nft/client/testutil"
	nfttypes "github.com/crypto-org-chain/chain-main/v2/x/nft/types"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	cfg := network.DefaultConfig()
	cfg.AppConstructor = app.Constructor
	cfg.NumValidators = 2

	s.cfg = cfg
	s.network = network.New(s.T(), cfg)

	_, err := s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestNft() {
	val := s.network.Validators[0]

	// ---------------------------------------------------------------------------

	from := val.Address
	tokenName := "Kitty Token"
	tokenURI := "uri"
	tokenData := "data"
	tokenID := "kitty"
	// owner     := "owner"
	denomName := "name"
	denom := "denom"
	schema := "schema"
	baseURL := val.APIAddress

	//------test GetCmdIssueDenom()-------------
	args := []string{
		fmt.Sprintf("--%s=%s", nftcli.FlagDenomName, denomName),
		fmt.Sprintf("--%s=%s", nftcli.FlagSchema, schema),

		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}

	respType := proto.Message(&sdk.TxResponse{})
	expectedCode := uint32(0)

	bz, err := nfttestutil.IssueDenomExec(val.ClientCtx, from.String(), denom, args...)
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(bz.Bytes(), respType), bz.String())
	txResp := respType.(*sdk.TxResponse)
	s.Require().Equal(expectedCode, txResp.Code)

	denomID := gjson.Get(txResp.RawLog, "0.events.0.attributes.0.value").String()

	//------test GetCmdQueryDenom()-------------
	url := fmt.Sprintf("%s/chainmain/nft/denoms/%s", baseURL, denomID)
	resp, err := rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QueryDenomResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	denomItem := respType.(*nfttypes.QueryDenomResponse)
	s.Require().Equal(denomName, denomItem.Denom.Name)
	s.Require().Equal(schema, denomItem.Denom.Schema)

	//------test GetCmdQueryDenomByName()-------------
	url = fmt.Sprintf("%s/chainmain/nft/denoms/name/%s", baseURL, denomName)
	resp, err = rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QueryDenomResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	denomItem = respType.(*nfttypes.QueryDenomResponse)
	s.Require().Equal(denomID, denomItem.Denom.Id)
	s.Require().Equal(denomName, denomItem.Denom.Name)
	s.Require().Equal(schema, denomItem.Denom.Schema)

	//------test GetCmdQueryDenoms()-------------
	url = fmt.Sprintf("%s/chainmain/nft/denoms", baseURL)
	resp, err = rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QueryDenomsResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	denomsResp := respType.(*nfttypes.QueryDenomsResponse)
	s.Require().Equal(1, len(denomsResp.Denoms))
	s.Require().Equal(denomID, denomsResp.Denoms[0].Id)

	//------test GetCmdMintNFT()-------------
	args = []string{
		fmt.Sprintf("--%s=%s", nftcli.FlagTokenData, tokenData),
		fmt.Sprintf("--%s=%s", nftcli.FlagRecipient, from.String()),
		fmt.Sprintf("--%s=%s", nftcli.FlagTokenURI, tokenURI),
		fmt.Sprintf("--%s=%s", nftcli.FlagTokenName, tokenName),

		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}

	respType = proto.Message(&sdk.TxResponse{})

	bz, err = nfttestutil.MintNFTExec(val.ClientCtx, from.String(), denomID, tokenID, args...)
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(bz.Bytes(), respType), bz.String())
	txResp = respType.(*sdk.TxResponse)
	s.Require().Equal(expectedCode, txResp.Code)

	//------test GetCmdQuerySupply()-------------
	url = fmt.Sprintf("%s/chainmain/nft/collections/%s/supply", baseURL, denomID)
	resp, err = rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QuerySupplyResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	supplyResp := respType.(*nfttypes.QuerySupplyResponse)
	s.Require().Equal(uint64(1), supplyResp.Amount)

	//------test GetCmdQueryNFT()-------------
	url = fmt.Sprintf("%s/chainmain/nft/nfts/%s/%s", baseURL, denomID, tokenID)
	resp, err = rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QueryNFTResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	nftItem := respType.(*nfttypes.QueryNFTResponse)
	s.Require().Equal(tokenID, nftItem.NFT.Id)
	s.Require().Equal(tokenName, nftItem.NFT.Name)
	s.Require().Equal(tokenURI, nftItem.NFT.URI)
	s.Require().Equal(tokenData, nftItem.NFT.Data)
	s.Require().Equal(from.String(), nftItem.NFT.Owner)

	//------test GetCmdQueryOwner()-------------
	url = fmt.Sprintf("%s/chainmain/nft/nfts?owner=%s", baseURL, from.String())
	resp, err = rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QueryOwnerResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	ownerResp := respType.(*nfttypes.QueryOwnerResponse)
	s.Require().Equal(from.String(), ownerResp.Owner.Address)
	s.Require().Equal(denom, ownerResp.Owner.IDCollections[0].DenomId)
	s.Require().Equal(tokenID, ownerResp.Owner.IDCollections[0].TokenIds[0])

	//------test GetCmdQueryCollection()-------------
	url = fmt.Sprintf("%s/chainmain/nft/collections/%s", baseURL, denomID)
	resp, err = rest.GetRequest(url)
	respType = proto.Message(&nfttypes.QueryCollectionResponse{})
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.JSONMarshaler.UnmarshalJSON(resp, respType))
	collectionResp := respType.(*nfttypes.QueryCollectionResponse)
	s.Require().Equal(1, len(collectionResp.Collection.NFTs))

}
