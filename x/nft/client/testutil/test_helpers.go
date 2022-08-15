// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)
package testutil

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/cli"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"

	"github.com/crypto-org-chain/chain-main/v4/app"
	nftcli "github.com/crypto-org-chain/chain-main/v4/x/nft/client/cli"

	pruningtypes "github.com/cosmos/cosmos-sdk/pruning/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	dbm "github.com/tendermint/tm-db"
)

func GetApp(val network.Validator) servertypes.Application {
	return app.New(
		val.Ctx.Logger, dbm.NewMemDB(), nil, true, make(map[int64]bool), val.Ctx.Config.RootDir, 0,
		app.MakeEncodingConfig(),
		simapp.EmptyAppOptions{},
		baseapp.SetPruning(pruningtypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
		baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices),
	)
}

func IssueDenomExec(clientCtx client.Context, from string, denom string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denom,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdIssueDenom(), args)
}

func BurnNFTExec(clientCtx client.Context, from string, denomID string, tokenID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomID,
		tokenID,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdBurnNFT(), args)
}

func MintNFTExec(clientCtx client.Context, from string, denomID string, tokenID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomID,
		tokenID,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdMintNFT(), args)
}

func EditNFTExec(clientCtx client.Context, from string, denomID string, tokenID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomID,
		tokenID,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdEditNFT(), args)
}

func TransferNFTExec(clientCtx client.Context, from string, recipient string, denomID string, tokenID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		recipient,
		denomID,
		tokenID,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdTransferNFT(), args)
}

func QueryDenomExec(clientCtx client.Context, denomID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomID,
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQueryDenom(), args)
}

func QueryDenomByNameExec(clientCtx client.Context, denomName string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomName,
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQueryDenomByName(), args)
}

func QueryCollectionExec(clientCtx client.Context, denomID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomID,
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQueryCollection(), args)
}

func QueryDenomsExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQueryDenoms(), args)
}

func QuerySupplyExec(clientCtx client.Context, denom string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denom,
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQuerySupply(), args)
}

func QueryOwnerExec(clientCtx client.Context, address string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		address,
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQueryOwner(), args)
}

func QueryNFTExec(clientCtx client.Context, denomID string, tokenID string, extraArgs ...string) (testutil.BufferWriter, error) {
	args := []string{
		denomID,
		tokenID,
		fmt.Sprintf("--%s=json", cli.OutputFlag),
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, nftcli.GetCmdQueryNFT(), args)
}
