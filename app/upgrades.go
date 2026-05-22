package app

import (
	"context"
	"fmt"
	"time"

	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
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
	baseAcc, ok := acc.(*authtypes.BaseAccount)
	if !ok {
		return fmt.Errorf("account at %s for module %q is %T, cannot convert to module account", addr, moduleName, acc)
	}
	macc := authtypes.NewModuleAccount(baseAcc, moduleName, perms...)
	if err := macc.Validate(); err != nil {
		return fmt.Errorf("module account %q: %w", moduleName, err)
	}
	ak.SetModuleAccount(ctx, macc)
	ctx.Logger().Info("converted base account to module account", "module", moduleName, "address", addr.String())
	return nil
}

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	app.registerV730UpgradeHandler()
}

func (app *ChainApp) registerV730UpgradeHandler() {
	planName := "v7.3.0"

	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		sdkCtx.Logger().Info("v7.3.0: exiting vested accounts positions...")
		if err := ExitVestedAccountsPositions(sdkCtx, app); err != nil {
			return map[string]uint64{}, fmt.Errorf("v7.3.0 exiting vested accounts positions: %w", err)
		}
		sdkCtx.Logger().Info("v7.3.0: exiting vested accounts positions completed")

		sdkCtx.Logger().Info("v7.3.0: running module migrations...")
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}
		sdkCtx.Logger().Info("v7.3.0: upgrade completed", "plan", plan.Name, "version_map", m)
		return m, nil
	})
}

func ExitVestedAccountsPositions(ctx sdk.Context, app *ChainApp) error {
	var toExitPositions []uint64
	if err := app.TieredRewardsKeeper.Positions.Walk(ctx, nil, func(posID uint64, pos tieredrewardstypes.Position) (bool, error) {
		ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
		if err != nil {
			return false, nil
		}
		acc := app.AccountKeeper.GetAccount(ctx, ownerAddr)
		if acc == nil {
			return false, nil
		}
		if _, ok := acc.(sdkvesting.VestingAccount); ok {
			toExitPositions = append(toExitPositions, posID)
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("walk positions: %w", err)
	}

	for _, posID := range toExitPositions {
		ctx.Logger().Info("v7.3.0 exiting vested accounts position", "position_id", posID)
		if err := app.TieredRewardsKeeper.ForceFullExitWithDelegation(ctx, posID); err != nil {
			return fmt.Errorf("force-exit position %d: %w", posID, err)
		}
	}

	ctx.Logger().Info("v7.3.0 exiting vested accounts positions: done", "positions_exited", len(toExitPositions))
	return nil
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
