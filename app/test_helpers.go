package app

import (
	"encoding/json"
	"time"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

// DefaultConsensusParams defines the default Tendermint consensus params used in
// SimApp testing.
var DefaultConsensusParams = &abci.ConsensusParams{
	Block: &abci.BlockParams{
		MaxBytes: 200000,
		MaxGas:   2000000,
	},
	Evidence: &tmproto.EvidenceParams{
		MaxAgeNumBlocks: 302400,
		MaxAgeDuration:  504 * time.Hour, // 3 weeks is the max duration
		MaxBytes:        10000,
	},
	Validator: &tmproto.ValidatorParams{
		PubKeyTypes: []string{
			tmtypes.ABCIPubKeyTypeEd25519,
		},
	},
}

func Constructor(val network.Validator) servertypes.Application {
	return New(
		val.Ctx.Logger, dbm.NewMemDB(), nil, true, make(map[int64]bool),
		val.Ctx.Config.RootDir, 0, MakeEncodingConfig(), emptyAppOptions{},
		bam.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
		bam.SetMinGasPrices(val.AppConfig.MinGasPrices),
	)
}

func setup(withGenesis bool, invCheckPeriod uint) (*ChainApp, GenesisState) {
	db := dbm.NewMemDB()
	encCdc := MakeEncodingConfig()
	app := New(log.NewNopLogger(), db, nil, true, map[int64]bool{}, DefaultNodeHome, invCheckPeriod, encCdc, emptyAppOptions{})
	if withGenesis {
		return app, NewDefaultGenesisState(encCdc.Marshaler)
	}
	return app, GenesisState{}
}

// Setup initializes a new ChainApp. A Nop logger is set in ChainApp.
func Setup(isCheckTx bool) *ChainApp {
	app, genesisState := setup(!isCheckTx, 5)
	if !isCheckTx {
		// init chain must be called to stop deliverState from being nil
		stateBytes, err := json.MarshalIndent(genesisState, "", " ")
		if err != nil {
			panic(err)
		}

		// Initialize the chain
		app.InitChain(
			abci.RequestInitChain{
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: DefaultConsensusParams,
				AppStateBytes:   stateBytes,
			},
		)
	}

	return app
}

// emptyAppOptions is a stub implementing AppOptions
type emptyAppOptions struct{}

// Get implements AppOptions
func (ao emptyAppOptions) Get(o string) interface{} {
	return nil
}

// createRandomAccounts is a strategy used by addTestAddrs() in order to generated addresses in random order.
func createRandomAccounts(accNum int) []sdk.AccAddress {
	testAddrs := make([]sdk.AccAddress, accNum)
	for i := 0; i < accNum; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		testAddrs[i] = sdk.AccAddress(pk.Address())
	}

	return testAddrs
}

type GenerateAccountStrategy func(int) []sdk.AccAddress

func addTestAddrs(app *ChainApp, ctx sdk.Context, accNum int, accAmt sdk.Coins, strategy GenerateAccountStrategy) []sdk.AccAddress {
	testAddrs := strategy(accNum)

	setTotalSupply(app, ctx, accAmt, accNum)

	// fill all the addresses with some coins, set the loose pool tokens simultaneously
	for _, addr := range testAddrs {
		saveAccount(app, ctx, addr, accAmt)
	}

	return testAddrs
}

// setTotalSupply provides the total supply based on accAmt * totalAccounts.
func setTotalSupply(app *ChainApp, ctx sdk.Context, accAmt sdk.Coins, totalAccounts int) {
	totalSupply := sdk.NewCoins()
	for i := 0; i < totalAccounts; i++ {
		totalSupply = totalSupply.Add(accAmt...)
	}
	prevSupply := app.BankKeeper.GetSupply(ctx)
	app.BankKeeper.SetSupply(ctx, banktypes.NewSupply(prevSupply.GetTotal().Add(totalSupply...)))
}

// saveAccount saves the provided account into the simapp with balance based on initCoins.
func saveAccount(app *ChainApp, ctx sdk.Context, addr sdk.AccAddress, initCoins sdk.Coins) {
	acc := app.AccountKeeper.NewAccountWithAddress(ctx, addr)
	app.AccountKeeper.SetAccount(ctx, acc)
	err := app.BankKeeper.AddCoins(ctx, addr, initCoins)
	if err != nil {
		panic(err)
	}
}

// AddTestAddrs constructs and returns accNum amount of accounts with an
// initial balance of accAmt in random order
func AddTestAddrs(app *ChainApp, ctx sdk.Context, accNum int, accAmt sdk.Coins) []sdk.AccAddress {
	return addTestAddrs(app, ctx, accNum, accAmt, createRandomAccounts)
}
