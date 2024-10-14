package app

import (
	"context"
	"fmt"
	"slices"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	planName := "v5.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return m, err
		}
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		{
			params := app.ICAHostKeeper.GetParams(sdkCtx)
			msg := "/ibc.applications.interchain_accounts.host.v1.MsgModuleQuerySafe"
			if !slices.ContainsFunc(params.AllowMessages, func(allowMsg string) bool {
				return allowMsg == "*" || allowMsg == msg
			}) {
				params.AllowMessages = append(params.AllowMessages, msg)
				app.ICAHostKeeper.SetParams(sdkCtx, params)
			}
		}
		return m, nil
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}
	if upgradeInfo.Name == planName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Deleted: []string{"icaauth"},
		}
		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
