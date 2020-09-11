package main

import (
	"os"

	"github.com/crypto-com/chain-main/cmd/chain-maind/cmd"
)

func main() {
	rootCmd, _ := cmd.NewRootCmd()
	if err := cmd.Execute(rootCmd); err != nil {
		os.Exit(1)
	}
}
