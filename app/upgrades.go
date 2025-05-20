package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	planName := "v6.0.0"
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
			if err := UpdateExpeditedParams(ctx, app.GovKeeper); err != nil {
				return m, err
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

func UpdateExpeditedParams(ctx context.Context, gov govkeeper.Keeper) error {
	govParams, err := gov.Params.Get(ctx)
	if err != nil {
		return err
	}
	if len(govParams.MinDeposit) > 0 {
		minDeposit := govParams.MinDeposit[0]
		expeditedAmount := minDeposit.Amount.MulRaw(govv1.DefaultMinExpeditedDepositTokensRatio)
		govParams.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(minDeposit.Denom, expeditedAmount))
	}
	threshold, err := sdkmath.LegacyNewDecFromStr(govParams.Threshold)
	if err != nil {
		return fmt.Errorf("invalid threshold string: %w", err)
	}
	expeditedThreshold, err := sdkmath.LegacyNewDecFromStr(govParams.ExpeditedThreshold)
	if err != nil {
		return fmt.Errorf("invalid expedited threshold string: %w", err)
	}
	if expeditedThreshold.LTE(threshold) {
		expeditedThreshold = threshold.Mul(DefaultThresholdRatio())
	}
	if expeditedThreshold.GT(sdkmath.LegacyOneDec()) {
		expeditedThreshold = sdkmath.LegacyOneDec()
	}
	govParams.ExpeditedThreshold = expeditedThreshold.String()
	if govParams.ExpeditedVotingPeriod != nil && govParams.VotingPeriod != nil && *govParams.ExpeditedVotingPeriod >= *govParams.VotingPeriod {
		votingPeriod := DurationToDec(*govParams.VotingPeriod)
		period := DecToDuration(DefaultPeriodRatio().Mul(votingPeriod))
		govParams.ExpeditedVotingPeriod = &period
	}
	if err := govParams.ValidateBasic(); err != nil {
		return err
	}
	return gov.Params.Set(ctx, govParams)
}

func DefaultThresholdRatio() sdkmath.LegacyDec {
	return govv1.DefaultExpeditedThreshold.Quo(govv1.DefaultThreshold)
}

func DefaultPeriodRatio() sdkmath.LegacyDec {
	return DurationToDec(govv1.DefaultExpeditedPeriod).Quo(DurationToDec(govv1.DefaultPeriod))
}

func DurationToDec(d time.Duration) sdkmath.LegacyDec {
	return sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("%f", d.Seconds()))
}

func DecToDuration(d sdkmath.LegacyDec) time.Duration {
	return time.Second * time.Duration(d.RoundInt64())
}
