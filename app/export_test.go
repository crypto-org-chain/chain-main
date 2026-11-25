package app_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestExportAppStateAndValidators(t *testing.T) {
	testCases := []struct {
		name          string
		forZeroHeight bool
		// expApplyHeight is the exported apply height of a rotation pending at lastHeight.
		expApplyHeight func(lastHeight int64) int64
	}{
		{
			name:          "for zero height",
			forZeroHeight: true,
			expApplyHeight: func(int64) int64 {
				return 1 + stakingtypes.ConsensusUpdateDelay
			},
		},
		{
			name:          "for non-zero height",
			forZeroHeight: false,
			expApplyHeight: func(lastHeight int64) int64 {
				return lastHeight + 1 + stakingtypes.ConsensusUpdateDelay
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := dbm.NewMemDB()
			chainApp := testutil.SetupWithDB(false, nil, db)
			_, err := chainApp.Commit()
			require.NoError(t, err)
			// A second block separates the zero-height apply height from the one a plain export would produce.
			_, err = chainApp.FinalizeBlock(&abci.RequestFinalizeBlock{
				Height: chainApp.LastBlockHeight() + 1,
				Hash:   chainApp.LastCommitID().Hash,
			})
			require.NoError(t, err)
			_, err = chainApp.Commit()
			require.NoError(t, err)

			// Making a new app object with the db, so that initchain hasn't been called
			chainApp2 := app.New(
				log.NewLogger(os.Stdout),
				db,
				true,
				simtestutil.NewAppOptionsWithFlagHome(app.DefaultNodeHome),
				baseapp.SetChainID(testutil.ChainID),
			)
			lastHeight := chainApp2.LastBlockHeight()
			require.GreaterOrEqual(t, lastHeight, int64(2))

			// The export reads the check state, so a rotation written there is pending in the exported state.
			ctx := chainApp2.NewContext(true).WithBlockHeader(tmproto.Header{Height: lastHeight, Time: time.Now().UTC()})
			validators, err := chainApp2.StakingKeeper.GetAllValidators(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, validators)
			valAddr, err := sdk.ValAddressFromBech32(validators[0].GetOperator())
			require.NoError(t, err)
			oldPubKey, err := validators[0].ConsPubKey()
			require.NoError(t, err)
			require.NoError(t, chainApp2.StakingKeeper.SetConsKeyRotation(ctx, valAddr, oldPubKey, ed25519.GenPrivKey().PubKey()))

			exported, err := chainApp2.ExportAppStateAndValidators(tc.forZeroHeight, []string{}, []string{})
			require.NoError(t, err)

			var genesis app.GenesisState
			require.NoError(t, json.Unmarshal(exported.AppState, &genesis))
			var stakingGenesis stakingtypes.GenesisState
			require.NoError(t, chainApp2.AppCodec().UnmarshalJSON(genesis[stakingtypes.ModuleName], &stakingGenesis))
			require.Len(t, stakingGenesis.PendingConsensusKeyRotations, 1)
			rotation := stakingGenesis.PendingConsensusKeyRotations[0]
			require.Equal(t, validators[0].GetOperator(), rotation.ValidatorAddress)
			require.Equal(t, tc.expApplyHeight(lastHeight), rotation.ApplyHeight)
		})
	}
}
