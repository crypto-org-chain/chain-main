package app

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	"github.com/crypto-org-chain/chain-main/v4/config"
)

type VestingAccountSpec struct {
	Account        string
	Periods        int
	PeriodDuration int64
}

var (
	MintAmount  = sdk.NewInt(7000000000000000000)
	VestingSpec = map[string]VestingAccountSpec{
		"chaintest": {
			Account:        "cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65",
			Periods:        60,
			PeriodDuration: 60,
		},
		"testnet-croeseid-4": {
			Account:        "tcro1t7y7hzl3spx8pdqfzsmw5u5n3y2fwg2nh9rngm",
			Periods:        60,
			PeriodDuration: 600,
		},
		"crypto-org-chain-mainnet-dryrun-1": {
			Account:        "cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65",
			Periods:        60,
			PeriodDuration: 600,
		},
		"crypto-org-chain-mainnet-1": {
			Account:        "cro198pra975lcj526974r80fflr6retphnl3l7f4h",
			Periods:        60,
			PeriodDuration: 2628000, // 1 month
		},
	}
)

func (app *ChainApp) RegisterUpgradeHandlers() {
	planName := "v5.0.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		vestingSpec, ok := VestingSpec[ctx.ChainID()]
		if !ok {
			return nil, fmt.Errorf("vesting spec not found for chainID %s", ctx.ChainID())
		}

		params := app.MintKeeper.GetParams(ctx)
		params.InflationMax = sdk.NewDecWithPrec(1, 2)  // 1%
		params.InflationMin = sdk.NewDecWithPrec(85, 4) // 0.85%
		app.MintKeeper.SetParams(ctx, params)

		// Mint 70B CRO
		totalAmount := sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, MintAmount))
		if err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, totalAmount); err != nil {
			return nil, fmt.Errorf("failed to mint coins: %w", err)
		}

		vestingAddr, err := sdk.AccAddressFromBech32(vestingSpec.Account)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vesting address: %w", err)
		}

		// Create vesting periods (60 periods)
		startTime := ctx.BlockTime()

		amountPerPeriod := MintAmount.QuoRaw(int64(vestingSpec.Periods))
		// the last period keep the remainder
		amountLastPeriod := MintAmount.Sub(amountPerPeriod.MulRaw(int64(vestingSpec.Periods - 1)))

		periods := make([]vesting.Period, vestingSpec.Periods)
		for i := 0; i < vestingSpec.Periods-1; i++ {
			periods[i] = vesting.Period{
				Length: vestingSpec.PeriodDuration,
				Amount: sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, amountPerPeriod)),
			}
		}
		periods[vestingSpec.Periods-1] = vesting.Period{
			Length: vestingSpec.PeriodDuration,
			Amount: sdk.NewCoins(sdk.NewCoin(config.BaseCoinUnit, amountLastPeriod)),
		}

		baseAcc := authtypes.NewBaseAccount(vestingAddr, nil, 0, 0)
		vestingAcc := vesting.NewPeriodicVestingAccount(
			baseAcc,
			totalAmount,
			startTime.Unix(),
			periods,
		)
		app.AccountKeeper.SetAccount(ctx, vestingAcc)

		if err := app.BankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			minttypes.ModuleName,
			vestingAddr,
			totalAmount,
		); err != nil {
			return nil, fmt.Errorf("failed to send coins to vesting account: %w", err)
		}

		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})
}
