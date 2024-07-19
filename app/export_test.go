package app_test

import (
	"os"
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/chain-main/v4/testutil"
	"github.com/stretchr/testify/require"
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
			ethApp := testutil.SetupWithDB(false, nil, db)
			ethApp.Commit()

			// Making a new app object with the db, so that initchain hasn't been called
			ethApp2 := app.New(
				log.NewLogger(os.Stdout),
				db,
				nil,
				true,
				simtestutil.NewAppOptionsWithFlagHome(app.DefaultNodeHome),
				baseapp.SetChainID(testutil.ChainID),
			)
			_, err := ethApp2.ExportAppStateAndValidators(false, []string{}, []string{})
			require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
		})
	}
}
