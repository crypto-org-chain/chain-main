// Copyright (c) 2016 Shanghai Bianjie AI Technology Inc.
// Modifications Copyright (c) 2020, Foris Limited (licensed under the Apache License, Version 2.0)
package rest

import (
	"github.com/gorilla/mux"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types/rest"
)

// RegisterHandlers registers the NFT REST routes.
func RegisterHandlers(cliCtx client.Context, r *mux.Router, queryRoute string) {
	registerQueryRoutes(cliCtx, r, queryRoute)
	registerTxRoutes(cliCtx, r, queryRoute)
}

const (
	RestParamDenomID = "denom-id"
	RestParamTokenID = "token-id"
	RestParamOwner   = "owner"
)

type issueDenomReq struct {
	BaseReq rest.BaseReq `json:"base_req"`
	Owner   string       `json:"owner"`
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Schema  string       `json:"schema"`
}

type mintNFTReq struct {
	BaseReq   rest.BaseReq `json:"base_req"`
	Owner     string       `json:"owner"`
	Recipient string       `json:"recipient"`
	DenomID   string       `json:"denom_id"`
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	URI       string       `json:"uri"`
	Data      string       `json:"data"`
}

type editNFTReq struct {
	BaseReq rest.BaseReq `json:"base_req"`
	Owner   string       `json:"owner"`
	Name    string       `json:"name"`
	URI     string       `json:"uri"`
	Data    string       `json:"data"`
}

type transferNFTReq struct {
	BaseReq   rest.BaseReq `json:"base_req"`
	Owner     string       `json:"owner"`
	Recipient string       `json:"recipient"`
	Name      string       `json:"name"`
	URI       string       `json:"uri"`
	Data      string       `json:"data"`
}

type burnNFTReq struct {
	BaseReq rest.BaseReq `json:"base_req"`
	Owner   string       `json:"owner"`
}
