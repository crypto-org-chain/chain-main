//go:build rocksdb

package app_test

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v8/app"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	simcli "github.com/cosmos/cosmos-sdk/x/simulation/client/cli"
)

func TestVersionDB(t *testing.T) {
	db := dbm.NewMemDB()

	appOptions := make(simtestutil.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = app.DefaultNodeHome
	appOptions[server.FlagInvCheckPeriod] = simcli.FlagPeriodValue //nolint:staticcheck
	appOptions["versiondb.enable"] = true
	logger := log.NewNopLogger()
	_ = app.New(logger, db, nil, false, appOptions, nil...)
}
