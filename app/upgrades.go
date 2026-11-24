package app

import (
	"context"
	"fmt"
	"time"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const (
	UpgradeV9PlanName = "v9"

	// x/group was removed in v9; its store key outlives the package.
	groupStoreKey = "group"
)

func EnsureModuleAccountIfExists(ctx sdk.Context, ak authkeeper.AccountKeeper, moduleName string, perms ...string) error {
	addr := ak.GetModuleAddress(moduleName)
	if addr == nil {
		return fmt.Errorf("module %q is not registered in maccPerms", moduleName)
	}
	acc := ak.GetAccount(ctx, addr)
	// creation of module account should be handled by the module itself
	if acc == nil {
		return nil
	}
	if _, ok := acc.(sdk.ModuleAccountI); ok {
		return nil
	}
	// Any other type converts too: anyone can create a vesting account here before the upgrade, and no key controls a module address.
	baseAcc := authtypes.NewBaseAccount(addr, nil, acc.GetAccountNumber(), acc.GetSequence())
	macc := authtypes.NewModuleAccount(baseAcc, moduleName, perms...)
	if err := macc.Validate(); err != nil {
		return fmt.Errorf("module account %q: %w", moduleName, err)
	}
	ak.SetModuleAccount(ctx, macc)
	ctx.Logger().Info("converted account to module account", "module", moduleName, "address", addr.String(), "previous_type", fmt.Sprintf("%T", acc))
	return nil
}

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	app.registerV9UpgradeHandler()
}

// The v9 binary must replace v8 at one height: its CometBFT accepts ML-DSA-65 keys and larger
// commit signatures that v8 rejects, so mixed binaries diverge on the same block.
func (app *ChainApp) registerV9UpgradeHandler() {
	app.UpgradeKeeper.SetUpgradeHandler(UpgradeV9PlanName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		// The pool address was not blocked before v9; staking panics on burn if a plain account sits there.
		if err := EnsureModuleAccountIfExists(sdkCtx, app.AccountKeeper, stakingtypes.KeyRotationFeePoolName, authtypes.Burner); err != nil {
			return map[string]uint64{}, fmt.Errorf("v9 key rotation fee pool: %w", err)
		}

		sdkCtx.Logger().Info("v9: running module migrations...")
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("v9: upgrade completed", "plan", plan.Name, "version_map", m)
		return m, nil
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}
	if upgradeInfo.Name == UpgradeV9PlanName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		// memiavl refuses to load a tree that is no longer mounted, so x/group's store must be deleted.
		storeUpgrades := storetypes.StoreUpgrades{Deleted: []string{groupStoreKey}}
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
	threshold, err := math.LegacyNewDecFromStr(govParams.Threshold)
	if err != nil {
		return fmt.Errorf("invalid threshold string: %w", err)
	}
	expeditedThreshold, err := math.LegacyNewDecFromStr(govParams.ExpeditedThreshold)
	if err != nil {
		return fmt.Errorf("invalid expedited threshold string: %w", err)
	}
	if expeditedThreshold.LTE(threshold) {
		expeditedThreshold = threshold.Mul(DefaultThresholdRatio())
	}
	if expeditedThreshold.GT(math.LegacyOneDec()) {
		expeditedThreshold = math.LegacyOneDec()
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

func DefaultThresholdRatio() math.LegacyDec {
	return govv1.DefaultExpeditedThreshold.Quo(govv1.DefaultThreshold)
}

func DefaultPeriodRatio() math.LegacyDec {
	return DurationToDec(govv1.DefaultExpeditedPeriod).Quo(DurationToDec(govv1.DefaultPeriod))
}

func DurationToDec(d time.Duration) math.LegacyDec {
	return math.LegacyMustNewDecFromStr(fmt.Sprintf("%f", d.Seconds()))
}

func DecToDuration(d math.LegacyDec) time.Duration {
	return time.Second * time.Duration(d.RoundInt64())
}
