package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	maxsupplytypes "github.com/crypto-org-chain/chain-main/v4/x/maxsupply/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// CircuitSuperAdmins maps chain IDs to their super admin addresses
var CircuitSuperAdmins = map[string][]string{
	"chaintest": {
		"cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65",
		"cro1sjcrmp0ngft2n2r3r4gcva4llfj8vjdnefdg4m", // ecosystem
	},
	"testnet-croeseid-4": {
		"tcro14thaw89nlpp8hjm83z6zp3w5ymlpgg2zejncw7",
		"tcro19uhea66tnx78r5258sq5vdad8msk47w6vn8f06",
	},
	"crypto-org-chain-mainnet-dryrun-1": {
		"cro1h704kvqdh48jzge7vvxpej9d6r9usvssehmxac",
		"cro1gv6e77tq7l06904g9nuu4nvnwcynaannwjpuaj",
		"cro160rhmah7kmfy9vg9jklkdqyv6nu9j7jnjpun9j",
	},
	"crypto-org-chain-mainnet-1": {
		"cro160rhmah7kmfy9vg9jklkdqyv6nu9j7jnjpun9j",
	},
}

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	// Register upgrade plan name for mainnet v7.0.0. If it's for testnet, add postfix "-testnet" to the plan name.
	planName := "v7.0.0"

	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		sdkCtx.Logger().Info("start to run module migrations...")

		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}

		var _ok bool
		maxSupplyParams := maxsupplytypes.DefaultParams()
		// update max supply to 100B * 10^18 basecro
		maxSupplyParams.MaxSupply, _ok = sdkmath.NewIntFromString("100000000000000000000000000000")
		if !_ok {
			return map[string]uint64{}, fmt.Errorf("invalid max supply")
		}

		if strings.Contains(planName, "testnet") {
			// For testnet, use different burned addresses or configuration
			maxSupplyParams.BurnedAddresses = []string{
				"tcro1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq9dpzma",
			}
		} else {
			// For mainnet
			maxSupplyParams.BurnedAddresses = []string{
				"cro1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqtcgxmv",
			}
		}

		if err := app.MaxSupplyKeeper.SetParams(sdkCtx, maxSupplyParams); err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("maxsupply module initialized with params",
			"max_supply", maxSupplyParams.MaxSupply.String(),
			"burned_addresses", maxSupplyParams.BurnedAddresses)

		m[maxsupplytypes.ModuleName] = 1

		sdkCtx.Logger().Info("upgrade completed",
			"plan", plan.Name,
			"version_map", m)

		return m, nil
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}
	if upgradeInfo.Name == planName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{
				maxsupplytypes.StoreKey,
			},
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
