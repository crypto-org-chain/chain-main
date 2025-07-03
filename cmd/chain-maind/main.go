package main

import (
	"os"

	"github.com/crypto-org-chain/chain-main/v4/app"
	cmd "github.com/crypto-org-chain/chain-main/v4/cmd/chain-maind/app"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func main() {
	rootCmd, _ := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
