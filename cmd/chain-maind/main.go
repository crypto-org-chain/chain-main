package main

import (
	"os"

	"github.com/crypto-com/chain-main/cmd/chain-maind/app"
)

func main() {
	rootCmd, _ := app.NewRootCmd()
	if err := app.Execute(rootCmd); err != nil {
		os.Exit(1)
	}
}
