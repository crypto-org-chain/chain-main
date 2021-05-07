// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021, CRO Protocol Labs ("Crypto.org") (licensed under the Apache License, Version 2.0)
package rest

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"

	"github.com/crypto-org-chain/chain-main/v2/x/nft/types"
)

func registerTxRoutes(cliCtx client.Context, r *mux.Router, queryRoute string) {
	// Issue a denom
	r.HandleFunc("/nft/nfts/denoms/issue", issueDenomHandlerFn(cliCtx)).Methods("POST")
	// Mint an NFT
	r.HandleFunc("/nft/nfts/mint", mintNFTHandlerFn(cliCtx)).Methods("POST")
	// Update an NFT
	r.HandleFunc(fmt.Sprintf("/nft/nfts/{%s}/{%s}", RestParamDenomID, RestParamTokenID), editNFTHandlerFn(cliCtx)).Methods("PUT")
	// Transfer an NFT to an address
	r.HandleFunc(fmt.Sprintf("/nft/nfts/{%s}/{%s}/transfer", RestParamDenomID, RestParamTokenID), transferNFTHandlerFn(cliCtx)).Methods("POST")
	// Burn an NFT
	r.HandleFunc(fmt.Sprintf("/nft/nfts/{%s}/{%s}/burn", RestParamDenomID, RestParamTokenID), burnNFTHandlerFn(cliCtx)).Methods("POST")
}

func issueDenomHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req issueDenomReq
		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}
		baseReq := req.BaseReq.Sanitize()
		if !baseReq.ValidateBasic(w) {
			return
		}

		// create the message
		msg := types.NewMsgIssueDenom(req.ID, req.Name, req.Schema, req.Owner)
		if err := msg.ValidateBasic(); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		tx.WriteGeneratedTxResponse(cliCtx, w, req.BaseReq, msg)
	}
}

func mintNFTHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req mintNFTReq
		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}
		baseReq := req.BaseReq.Sanitize()
		if !baseReq.ValidateBasic(w) {
			return
		}

		if req.Recipient == "" {
			req.Recipient = req.Owner
		}
		// create the message
		msg := types.NewMsgMintNFT(
			req.ID,
			req.DenomID,
			req.Name,
			req.URI,
			req.Data,
			req.Owner,
			req.Recipient,
		)
		if err := msg.ValidateBasic(); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		tx.WriteGeneratedTxResponse(cliCtx, w, req.BaseReq, msg)
	}
}

func editNFTHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req editNFTReq
		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}
		baseReq := req.BaseReq.Sanitize()
		if !baseReq.ValidateBasic(w) {
			return
		}

		vars := mux.Vars(r)
		// create the message
		msg := types.NewMsgEditNFT(
			vars[RestParamTokenID],
			vars[RestParamDenomID],
			req.Name,
			req.URI,
			req.Data, req.Owner,
		)
		if err := msg.ValidateBasic(); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		tx.WriteGeneratedTxResponse(cliCtx, w, req.BaseReq, msg)
	}
}

func transferNFTHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req transferNFTReq
		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}
		baseReq := req.BaseReq.Sanitize()
		if !baseReq.ValidateBasic(w) {
			return
		}
		if _, err := sdk.AccAddressFromBech32(req.Recipient); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		vars := mux.Vars(r)
		// create the message
		msg := types.NewMsgTransferNFT(
			vars[RestParamTokenID],
			vars[RestParamDenomID],
			req.Owner,
			req.Recipient,
		)
		if err := msg.ValidateBasic(); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		tx.WriteGeneratedTxResponse(cliCtx, w, req.BaseReq, msg)
	}
}

func burnNFTHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req burnNFTReq
		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}
		baseReq := req.BaseReq.Sanitize()
		if !baseReq.ValidateBasic(w) {
			return
		}

		vars := mux.Vars(r)

		// create the message
		msg := types.NewMsgBurnNFT(
			req.Owner,
			vars[RestParamTokenID],
			vars[RestParamDenomID],
		)
		if err := msg.ValidateBasic(); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		tx.WriteGeneratedTxResponse(cliCtx, w, req.BaseReq, msg)
	}
}
