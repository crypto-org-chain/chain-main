package testutil

import (
	"encoding/json"
	"time"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/crypto-org-chain/chain-main/v4/app"
)

// DefaultConsensusParams defines the default Tendermint consensus params used in
// ChainApp testing.
var DefaultConsensusParams = &cmtproto.ConsensusParams{
	Block: &cmtproto.BlockParams{
		MaxBytes: 1048576,
		MaxGas:   81500000, // default limit
	},
	Evidence: &cmtproto.EvidenceParams{
		MaxAgeNumBlocks: 302400,
		MaxAgeDuration:  504 * time.Hour, // 3 weeks is the max duration
		MaxBytes:        10000,
	},
	Validator: &cmtproto.ValidatorParams{
		PubKeyTypes: []string{
			tmtypes.ABCIPubKeyTypeEd25519,
		},
	},
}

// Setup initializes a new ChainApp. A Nop logger is set in ChainApp.
func Setup(isCheckTx bool, patch func(*app.ChainApp, app.GenesisState) app.GenesisState) *app.ChainApp {
	return SetupWithDB(isCheckTx, patch, dbm.NewMemDB())
}

func SetupWithOpts(
	isCheckTx bool,
	patch func(*app.ChainApp, app.GenesisState) app.GenesisState,
	appOptions simtestutil.AppOptionsMap,
) *app.ChainApp {
	return SetupWithDBAndOpts(isCheckTx, patch, dbm.NewMemDB(), appOptions)
}

const ChainID = "chainmain-1"

func SetupWithDB(isCheckTx bool, patch func(*app.ChainApp, app.GenesisState) app.GenesisState, db dbm.DB) *app.ChainApp {
	return SetupWithDBAndOpts(isCheckTx, patch, db, nil)
}

// SetupWithDBAndOpts initializes a new ChainApp. A Nop logger is set in ChainApp.
func SetupWithDBAndOpts(
	isCheckTx bool,
	patch func(*app.ChainApp, app.GenesisState) app.GenesisState,
	db dbm.DB,
	appOptions simtestutil.AppOptionsMap,
) *app.ChainApp {
	if appOptions == nil {
		appOptions = make(simtestutil.AppOptionsMap, 0)
	}
	appOptions[server.FlagInvCheckPeriod] = 5
	appOptions[flags.FlagHome] = app.DefaultNodeHome
	app := app.New(log.NewNopLogger(),
		db,
		nil,
		true,
		appOptions,
		baseapp.SetChainID(ChainID),
	)

	if !isCheckTx {
		// init chain must be called to stop deliverState from being nil
		genesisState := NewTestGenesisState(app.AppCodec(), app.DefaultGenesis())
		if patch != nil {
			genesisState = patch(app, genesisState)
		}

		stateBytes, err := json.MarshalIndent(genesisState, "", " ")
		if err != nil {
			panic(err)
		}

		// Initialize the chain
		consensusParams := DefaultConsensusParams
		initialHeight := app.LastBlockHeight() + 1
		consensusParams.Abci = &cmtproto.ABCIParams{VoteExtensionsEnableHeight: initialHeight}
		if _, err := app.InitChain(
			&abci.RequestInitChain{
				ChainId:         ChainID,
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: consensusParams,
				AppStateBytes:   stateBytes,
				InitialHeight:   initialHeight,
			},
		); err != nil {
			panic(err)
		}
	}
	if _, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: app.LastBlockHeight() + 1,
		Hash:   app.LastCommitID().Hash,
	}); err != nil {
		panic(err)
	}
	return app
}

func StateFn(a *app.ChainApp) simtypes.AppStateFn {
	return simtestutil.AppStateFnWithExtendedCb(
		a.AppCodec(), a.SimulationManager(), a.DefaultGenesis(), nil,
	)
}
