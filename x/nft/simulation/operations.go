// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/crypto-org-chain/chain-main/v4/x/nft/keeper"
	"github.com/crypto-org-chain/chain-main/v4/x/nft/types"
)

// Simulation operation weights constants
// nolint:gosec
const (
	OpWeightMsgMintNFT     = "op_weight_msg_mint_nft"
	OpWeightMsgEditNFT     = "op_weight_msg_edit_nft_tokenData"
	OpWeightMsgTransferNFT = "op_weight_msg_transfer_nft"
	OpWeightMsgBurnNFT     = "op_weight_msg_transfer_burn_nft"
)

// WeightedOperations returns all the operations from the module with their respective weights
func WeightedOperations(
	appParams simtypes.AppParams,
	cdc codec.JSONCodec,
	k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper) simulation.WeightedOperations {

	var weightMint, weightEdit, weightBurn, weightTransfer int
	appParams.GetOrGenerate(
		cdc, OpWeightMsgMintNFT, &weightMint, nil,
		func(_ *rand.Rand) {
			weightMint = 100
		},
	)

	appParams.GetOrGenerate(
		cdc, OpWeightMsgEditNFT, &weightEdit, nil,
		func(_ *rand.Rand) {
			weightEdit = 50
		},
	)

	appParams.GetOrGenerate(
		cdc, OpWeightMsgTransferNFT, &weightTransfer, nil,
		func(_ *rand.Rand) {
			weightTransfer = 50
		},
	)

	appParams.GetOrGenerate(
		cdc, OpWeightMsgBurnNFT, &weightBurn, nil,
		func(_ *rand.Rand) {
			weightBurn = 10
		},
	)

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(
			weightMint,
			SimulateMsgMintNFT(k, ak, bk),
		),
		simulation.NewWeightedOperation(
			weightEdit,
			SimulateMsgEditNFT(k, ak, bk),
		),
		simulation.NewWeightedOperation(
			weightTransfer,
			SimulateMsgTransferNFT(k, ak, bk),
		),
		simulation.NewWeightedOperation(
			weightBurn,
			SimulateMsgBurnNFT(k, ak, bk),
		),
	}
}

// SimulateMsgTransferNFT simulates the transfer of an NFT
func SimulateMsgTransferNFT(k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (
		opMsg simtypes.OperationMsg, fOps []simtypes.FutureOperation, err error,
	) {
		ownerAddr, denom, nftID := getRandomNFTFromOwner(ctx, k, r)
		if ownerAddr.Empty() {
			err = fmt.Errorf("invalid account")
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeTransfer, err.Error()), nil, err
		}

		recipientAccount, _ := simtypes.RandomAcc(r, accs)
		msg := types.NewMsgTransferNFT(
			nftID,
			denom,
			ownerAddr.String(),                // sender
			recipientAccount.Address.String(), // recipient
		)

		simAccount, found := simtypes.FindAccount(accs, ownerAddr)
		if !found {
			err = fmt.Errorf("account %s not found", msg.Sender)
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeTransfer, err.Error()), nil, err
		}

		spendable := bk.SpendableCoins(ctx, simAccount.Address)
		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           simappparams.MakeTestEncodingConfig().TxConfig,
			Cdc:             nil,
			Msg:             msg,
			MsgType:         msg.Type(),
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: spendable,
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgEditNFT simulates an edit tokenData transaction
func SimulateMsgEditNFT(k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (
		opMsg simtypes.OperationMsg, fOps []simtypes.FutureOperation, err error,
	) {
		ownerAddr, denom, nftID, err := getRandomNFTFromOwnerAndCreator(ctx, k, r)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeEditNFT, err.Error()), nil, err
		}

		if ownerAddr.Empty() {
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeEditNFT, "denom creator does not own any NFTs"), nil, nil
		}

		msg := types.NewMsgEditNFT(
			nftID,
			denom,
			"",
			simtypes.RandStringOfLength(r, 45), // tokenURI
			simtypes.RandStringOfLength(r, 10), // tokenData
			ownerAddr.String(),
		)

		simAccount, found := simtypes.FindAccount(accs, ownerAddr)
		if !found {
			err = fmt.Errorf("account %s not found", ownerAddr)
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeEditNFT, err.Error()), nil, err
		}
		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           simappparams.MakeTestEncodingConfig().TxConfig,
			Cdc:             nil,
			Msg:             msg,
			MsgType:         msg.Type(),
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.Coins{},
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgMintNFT simulates a mint of an NFT
func SimulateMsgMintNFT(k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (
		opMsg simtypes.OperationMsg, fOps []simtypes.FutureOperation, err error,
	) {
		denom, err := k.GetDenom(ctx, getRandomDenom(ctx, k, r))
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeMintNFT, err.Error()), nil, err
		}

		randomSenderAddress, _ := sdk.AccAddressFromBech32(denom.Creator) // nolint: errcheck
		randomRecipient, _ := simtypes.RandomAcc(r, accs)

		simAccount, found := simtypes.FindAccount(accs, randomSenderAddress)
		if !found {
			err = fmt.Errorf("account %s not found", randomSenderAddress)
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeMintNFT, err.Error()), nil, err
		}

		msg := types.NewMsgMintNFT(
			RandnNFTID(r, types.MinDenomLen, types.MaxDenomLen), // nft ID
			denom.Id, // denom
			"",
			simtypes.RandStringOfLength(r, 45), // tokenURI
			simtypes.RandStringOfLength(r, 10), // tokenData
			randomSenderAddress.String(),       // sender
			randomRecipient.Address.String(),   // recipient
		)

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           simappparams.MakeTestEncodingConfig().TxConfig,
			Cdc:             nil,
			Msg:             msg,
			MsgType:         msg.Type(),
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.Coins{},
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgBurnNFT simulates a burn of an existing NFT
func SimulateMsgBurnNFT(k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (
		opMsg simtypes.OperationMsg, fOps []simtypes.FutureOperation, err error,
	) {
		ownerAddr, denom, nftID, err := getRandomNFTFromOwnerAndCreator(ctx, k, r)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeBurnNFT, err.Error()), nil, err
		}

		if ownerAddr.Empty() {
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeBurnNFT, "denom creator does not own any NFTs"), nil, nil
		}

		simAccount, found := simtypes.FindAccount(accs, ownerAddr)
		if !found {
			err = fmt.Errorf("account %s not found", ownerAddr)
			return simtypes.NoOpMsg(types.ModuleName, types.EventTypeBurnNFT, err.Error()), nil, err
		}

		msg := types.NewMsgBurnNFT(ownerAddr.String(), nftID, denom)

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           simappparams.MakeTestEncodingConfig().TxConfig,
			Cdc:             nil,
			Msg:             msg,
			MsgType:         msg.Type(),
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.Coins{},
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

func getRandomNFTFromOwner(ctx sdk.Context, k keeper.Keeper, r *rand.Rand) (address sdk.AccAddress, denomID, tokenID string) {
	owners, _ := k.GetOwners(ctx) // nolint: errcheck

	ownersLen := len(owners)
	if ownersLen == 0 {
		return nil, "", ""
	}

	// get random owner
	i := r.Intn(ownersLen)
	owner := owners[i]

	idCollectionsLen := len(owner.IDCollections)
	if idCollectionsLen == 0 {
		return nil, "", ""
	}

	// get random collection from owner's balance
	i = r.Intn(idCollectionsLen)
	idCollection := owner.IDCollections[i] // nfts IDs
	denomID = idCollection.DenomId

	idsLen := len(idCollection.TokenIds)
	if idsLen == 0 {
		return nil, "", ""
	}

	// get random nft from collection
	i = r.Intn(idsLen)
	tokenID = idCollection.TokenIds[i]

	ownerAddress, _ := sdk.AccAddressFromBech32(owner.Address) // nolint: errcheck
	return ownerAddress, denomID, tokenID
}

func getRandomNFTFromOwnerAndCreator(ctx sdk.Context, k keeper.Keeper, r *rand.Rand) (address sdk.AccAddress, denomID, tokenID string, err error) {
	denom, err := k.GetDenom(ctx, getRandomDenom(ctx, k, r))
	if err != nil {
		return nil, "", "", err
	}

	creator, _ := sdk.AccAddressFromBech32(denom.Creator) // nolint: errcheck

	owner, err := k.GetOwner(ctx, creator, denom.Id)
	if err != nil {
		return nil, "", "", err
	}

	idCollectionsLen := len(owner.IDCollections)
	if idCollectionsLen == 0 {
		return nil, "", "", nil
	}

	// get random collection from owner's balance
	i := r.Intn(idCollectionsLen)
	idCollection := owner.IDCollections[i] // nfts IDs
	denomID = idCollection.DenomId

	idsLen := len(idCollection.TokenIds)
	if idsLen == 0 {
		return nil, "", "", nil
	}

	// get random nft from collection
	i = r.Intn(idsLen)
	tokenID = idCollection.TokenIds[i]

	ownerAddress, _ := sdk.AccAddressFromBech32(owner.Address) // nolint: errcheck
	return ownerAddress, denomID, tokenID, nil
}

func getRandomDenom(ctx sdk.Context, k keeper.Keeper, r *rand.Rand) string {
	var denoms = []string{kitties, doggos}
	i := r.Intn(len(denoms))
	return denoms[i]
}
