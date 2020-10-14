package main

import (
	"os"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/crypto-com/chain-main/cmd/chain-maind/app"
)

func main() {
	rootCmd, _ := app.NewRootCmd()
	if err := app.Execute(rootCmd); err != nil {
		switch e := err.(type) {
		case server.ErrorCode:
			os.Exit(e.Code)
		default:
			os.Exit(1)
		}
	}
}
