// +build testbincover

package main

import (
	"testing"

	"github.com/confluentinc/bincover"
	"github.com/cosmos/cosmos-sdk/server"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/crypto-com/chain-main/app"
	cmd "github.com/crypto-com/chain-main/cmd/chain-maind/app"
)

func test_main() {
	rootCmd, _ := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, app.DefaultNodeHome); err != nil {
		switch e := err.(type) {
		case server.ErrorCode:
			bincover.ExitCode = e.Code
		default:
			bincover.ExitCode = 1
		}
	}
}

// TestBincoverRunMain wrap main in test function to have coverage support
// https://www.confluent.io/blog/measure-go-code-coverage-with-bincover/
func TestBincoverRunMain(t *testing.T) {
	bincover.RunTest(test_main)
}
