package app

import (
	"encoding/json"
	"fmt"
	"log"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	storetypes "cosmossdk.io/store/types"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ExportAppStateAndValidators exports the state of the application for a genesis
// file.
func (app *ChainApp) ExportAppStateAndValidators(
	forZeroHeight bool, jailAllowedAddrs, modulesToExport []string,
) (servertypes.ExportedApp, error) {
	// as if they could withdraw from the start of the next block
	ctx := app.NewContext(true).WithBlockHeader(tmproto.Header{Height: app.LastBlockHeight()})

	// We export at last height + 1, because that's the height at which
	// Tendermint will start InitChain.
	height := app.LastBlockHeight() + 1

	if forZeroHeight {
		height = 0
		err := app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs)
		if err != nil {
			return servertypes.ExportedApp{}, err
		}
	}

	genState, err := app.ModuleManager.ExportGenesisForModules(ctx, app.appCodec, modulesToExport)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	validators, err := staking.WriteValidators(ctx, app.StakingKeeper)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          height,
		ConsensusParams: app.GetConsensusParams(ctx),
	}, nil
}

// prepare for fresh start at zero height
// NOTE zero height genesis is a temporary feature which will be deprecated
//
//	in favor of export at a block height
func (app *ChainApp) prepForZeroHeightGenesis(ctx sdk.Context, jailAllowedAddrs []string) error {
	applyAllowedAddrs := len(jailAllowedAddrs) > 0

	// check if there is a allowed address list

	allowedAddrsMap := make(map[string]bool)

	for _, addr := range jailAllowedAddrs {
		_, err := sdk.ValAddressFromBech32(addr)
		if err != nil {
			log.Fatal(err)
		}
		allowedAddrsMap[addr] = true
	}

	/* Handle fee distribution state. */

	// withdraw all validator commission
	err := app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valBz, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			panic(err)
		}
		_, err = app.DistrKeeper.WithdrawValidatorCommission(ctx, valBz)
		if err != nil {
			log := app.DistrKeeper.Logger(ctx)
			log.Error(fmt.Sprintf("Withdraw validator commission: %v", err))
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// withdraw all delegator rewards
	dels, err := app.StakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		return err
	}
	for _, delegation := range dels {
		valAddr, errv := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if errv != nil {
			panic(errv)
		}

		delAddr, errd := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if errd != nil {
			panic(errd)
		}
		_, err := app.DistrKeeper.WithdrawDelegationRewards(ctx, delAddr, valAddr)
		if err != nil {
			log := app.DistrKeeper.Logger(ctx)
			log.Error(fmt.Sprintf("Withdraw delegation rewards: %v", err))
		}
	}

	// clear validator slash events
	app.DistrKeeper.DeleteAllValidatorSlashEvents(ctx)

	// clear validator historical rewards
	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	// set context height to zero
	height := ctx.BlockHeight()
	ctx = ctx.WithBlockHeight(0)

	// reinitialize all validators
	err = app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valBz, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			panic(err)
		}
		// donate any unwithdrawn outstanding reward fraction tokens to the community pool
		scraps, err := app.DistrKeeper.GetValidatorOutstandingRewardsCoins(ctx, valBz)
		if err != nil {
			panic(err)
		}
		feePool, err := app.DistrKeeper.FeePool.Get(ctx)
		if err != nil {
			panic(err)
		}
		feePool.CommunityPool = feePool.CommunityPool.Add(scraps...)
		if err := app.DistrKeeper.FeePool.Set(ctx, feePool); err != nil {
			panic(err)
		}

		if err := app.DistrKeeper.Hooks().AfterValidatorCreated(ctx, valBz); err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// reinitialize all delegations
	for _, del := range dels {
		valAddr, errv := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if errv != nil {
			panic(errv)
		}

		delAddr, errd := sdk.AccAddressFromBech32(del.DelegatorAddress)
		if errd != nil {
			panic(errd)
		}
		if err := app.DistrKeeper.Hooks().BeforeDelegationCreated(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
		if err := app.DistrKeeper.Hooks().AfterDelegationModified(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
	}

	// reset context height
	ctx = ctx.WithBlockHeight(height)

	/* Handle staking state. */

	// iterate through redelegations, reset creation height
	err = app.StakingKeeper.IterateRedelegations(ctx, func(_ int64, red stakingtypes.Redelegation) (stop bool) {
		for i := range red.Entries {
			red.Entries[i].CreationHeight = 0
		}
		err := app.StakingKeeper.SetRedelegation(ctx, red)
		if err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// iterate through unbonding delegations, reset creation height
	err = app.StakingKeeper.IterateUnbondingDelegations(ctx, func(_ int64, ubd stakingtypes.UnbondingDelegation) (stop bool) {
		for i := range ubd.Entries {
			ubd.Entries[i].CreationHeight = 0
		}
		err := app.StakingKeeper.SetUnbondingDelegation(ctx, ubd)
		if err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// Iterate through validators by power descending, reset bond heights
	store := ctx.KVStore(app.keys[stakingtypes.StoreKey])
	iter := storetypes.KVStoreReversePrefixIterator(store, stakingtypes.ValidatorsKey)

	for ; iter.Valid(); iter.Next() {
		addr := sdk.ValAddress(stakingtypes.AddressFromValidatorsKey(iter.Key()))
		validator, err := app.StakingKeeper.GetValidator(ctx, addr)
		if err != nil {
			return fmt.Errorf("expected validator %s not found, %w", addr, err)
		}

		validator.UnbondingHeight = 0
		if applyAllowedAddrs && !allowedAddrsMap[addr.String()] {
			validator.Jailed = true
		}

		err = app.StakingKeeper.SetValidator(ctx, validator)
		if err != nil {
			panic(err)
		}
	}

	if err := iter.Close(); err != nil {
		log.Fatal(err)
	}

	if _, err := app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx); err != nil {
		return err
	}

	/* Handle slashing state. */

	// reset start height on signing infos
	return app.SlashingKeeper.IterateValidatorSigningInfos(
		ctx,
		func(addr sdk.ConsAddress, info slashingtypes.ValidatorSigningInfo) (stop bool) {
			info.StartHeight = 0
			err := app.SlashingKeeper.SetValidatorSigningInfo(ctx, addr, info)
			if err != nil {
				panic(err)
			}
			return false
		},
	)
}
