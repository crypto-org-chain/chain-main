package testutil

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/cli"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	bankcli "github.com/cosmos/cosmos-sdk/x/bank/client/cli"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
	chainbankcli "github.com/crypto-com/chain-main/x/bank/client/cli"

	chainsdk "github.com/crypto-com/chain-main/x/chainmain/types"
)

func MsgSendExec(
	clientCtx client.Context,
	coinParser chainsdk.CoinParser,
	from, to, amount fmt.Stringer,
	extraArgs ...string,
) (testutil.BufferWriter, error) {
	args := []string{from.String(), to.String(), amount.String()}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, chainbankcli.NewSendTxCmd(coinParser), args)
}

func MsgMultiSend1ToManyExec(
	clientCtx client.Context,
	coinParser chainsdk.CoinParser,
	from fmt.Stringer,
	outputs []types.Output,
	extraArgs ...string,
) (testutil.BufferWriter, error) {
	args := []string{from.String()}
	for _, output := range outputs {
		args = append(args, output.Address, output.Coins.String())
	}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, chainbankcli.NewMultiSend1ToManyTxCmd(coinParser), args)
}

func QueryBalancesExec(
	clientCtx client.Context,
	address fmt.Stringer,
	extraArgs ...string,
) (testutil.BufferWriter, error) {
	args := []string{address.String(), fmt.Sprintf("--%s=json", cli.OutputFlag)}
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, bankcli.GetBalancesCmd(), args)
}
