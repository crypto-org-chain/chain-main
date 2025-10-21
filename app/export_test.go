package app_test

import (
	"os"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

func TestExportAppStateAndValidators(t *testing.T) {
	testCases := []struct {
		name          string
		forZeroHeight bool
	}{
		{
			"for zero height",
			true,
		},
		{
			"for non-zero height",
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := dbm.NewMemDB()
			chainApp := testutil.SetupWithDB(false, nil, db)
			_, err := chainApp.Commit()
			require.NoError(t, err, "ExportAppStateAndValidators should not have an error")

			// Making a new app object with the db, so that initchain hasn't been called
			chainApp2 := app.New(
				log.NewLogger(os.Stdout),
				db,
				nil,
				true,
				simtestutil.NewAppOptionsWithFlagHome(app.DefaultNodeHome),
				baseapp.SetChainID(testutil.ChainID),
			)
			_, err = chainApp2.ExportAppStateAndValidators(false, []string{}, []string{})
			require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
		})
	}
}
