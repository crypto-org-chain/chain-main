package app

import (
	"context"
	"fmt"
	"time"

	"cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

const UpgradeV8PlanName = "v8"

const UpgradeV9PlanName = "v9"

// PauseVestingDuration is how far an indefinite pause pushes the unvested
// remainder out: long enough to mean "until governance decides otherwise",
// nowhere near overflowing the int64 unix timestamps vesting uses.
const PauseVestingDuration int64 = 50 * 365 * 24 * 60 * 60

// PauseDetectionHorizon is the end time beyond which a schedule is treated as
// already paused. No legitimate reserve schedule runs this long, so an account
// ending past it can only have been pushed out by an earlier pause.
const PauseDetectionHorizon int64 = 30 * 365 * 24 * 60 * 60

// PauseVestingSpec maps chain ID to the reserve vesting account created by the
// v5.0.0 upgrade handler, whose vesting this pause suspends. Addresses are
// copied verbatim from that handler's VestingSpec. A chain absent from this map
// never ran v5.0.0 and is skipped.
var PauseVestingSpec = map[string]string{
	"crypto-org-chain-mainnet-1":        "cro198pra975lcj526974r80fflr6retphnl3l7f4h",
	"crypto-org-chain-mainnet-dryrun-1": "cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65",
	"testnet-croeseid-4":                "tcro1t7y7hzl3spx8pdqfzsmw5u5n3y2fwg2nh9rngm",
	"chaintest":                         "cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65",
}

// PauseReserveVesting suspends vesting on the reserve account for the current
// chain. Chains with no spec entry are skipped so that devnets which never ran
// the v5.0.0 handler are not blocked from upgrading; on a chain that does have
// an entry, a missing or unexpected account is a hard error.
func PauseReserveVesting(ctx sdk.Context, ak authkeeper.AccountKeeper) error {
	addrStr, ok := PauseVestingSpec[ctx.ChainID()]
	if !ok {
		ctx.Logger().Info("no reserve vesting spec for chain, skipping pause",
			"chain_id", ctx.ChainID())
		return nil
	}

	addr, err := sdk.AccAddressFromBech32(addrStr)
	if err != nil {
		return fmt.Errorf("invalid reserve vesting address %q for chain %s: %w",
			addrStr, ctx.ChainID(), err)
	}

	return PauseVestingAccount(ctx, ak, addr, PauseVestingDuration)
}

func PauseVestingAccount(ctx sdk.Context, ak authkeeper.AccountKeeper, addr sdk.AccAddress, delta int64) error {
	acc := ak.GetAccount(ctx, addr)
	pva, ok := acc.(*vestingtypes.PeriodicVestingAccount)
	if !ok {
		return fmt.Errorf("account %s is %T, expected a periodic vesting account", addr, acc)
	}

	now := ctx.BlockTime().Unix()

	// Already paused: the remainder is parked far beyond any real schedule.
	// Makes a re-run harmless instead of stacking a second shift. The horizon is
	// absolute rather than derived from delta, so it holds for any shift size.
	if pva.EndTime > now+PauseDetectionHorizon {
		ctx.Logger().Info("vesting already paused, skipping",
			"address", addr.String(), "end_time", pva.EndTime)
		return nil
	}

	// Locate the period currently in progress: the first one whose cumulative
	// end time is still in the future. Periods before it have fully vested and
	// must not be touched, so that nothing already unlocked is re-locked.
	idx := -1
	periodEnd := pva.StartTime
	for i, p := range pva.VestingPeriods {
		periodEnd += p.Length
		if periodEnd > now {
			idx = i
			break
		}
	}

	if idx < 0 {
		return fmt.Errorf("account %s is fully vested at %s, nothing to pause", addr, ctx.BlockTime())
	}

	// Stretching the in-progress period delays it and, because period start
	// times are cumulative, every period after it. Elapsed time within the
	// period is preserved rather than forfeited.
	vestedBefore := pva.GetVestedCoins(ctx.BlockTime())
	pva.VestingPeriods[idx].Length += delta
	pva.EndTime += delta

	// Validate re-derives EndTime from the period lengths, so it catches both a
	// mismatch between the two lines above and any int64 overflow in them.
	if err := pva.Validate(); err != nil {
		return fmt.Errorf("paused vesting account %s is invalid: %w", addr, err)
	}

	// The whole point of pausing is that it is prospective only. A structurally
	// valid schedule can still move the already-unlocked amount, so check it
	// directly rather than trusting the arithmetic above.
	if vestedAfter := pva.GetVestedCoins(ctx.BlockTime()); !vestedBefore.Equal(vestedAfter) {
		return fmt.Errorf(
			"pausing %s would change vested coins from %s to %s; refusing",
			addr, vestedBefore, vestedAfter,
		)
	}

	ak.SetAccount(ctx, pva)
	ctx.Logger().Info("paused vesting account",
		"address", addr.String(),
		"period_stretched", idx,
		"new_end_time", pva.EndTime,
		"still_vesting", pva.GetVestingCoins(ctx.BlockTime()).String(),
	)
	return nil
}

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
	app.registerV8UpgradeHandler()
	app.registerV9UpgradeHandler()
}

// registerV8UpgradeHandler registers the "v8" plan
func (app *ChainApp) registerV8UpgradeHandler() {
	app.UpgradeKeeper.SetUpgradeHandler(UpgradeV8PlanName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		sdkCtx.Logger().Info("v8: running module migrations...")
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("v8: upgrade completed", "plan", plan.Name, "version_map", m)
		return m, nil
	})
}

// registerV9UpgradeHandler registers the "v9" plan, which suspends vesting on
// the 70B CRO reserve account created by the v5.0.0 handler.
func (app *ChainApp) registerV9UpgradeHandler() {
	app.UpgradeKeeper.SetUpgradeHandler(UpgradeV9PlanName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		sdkCtx.Logger().Info("v9: pausing reserve vesting...")
		if err := PauseReserveVesting(sdkCtx, app.AccountKeeper); err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("v9: running module migrations...")
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("v9: upgrade completed", "plan", plan.Name, "version_map", m)
		return m, nil
	})
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
