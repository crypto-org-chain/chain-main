package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	inflationtypes "github.com/crypto-org-chain/chain-main/v8/x/inflation/types"
	nfttypes "github.com/crypto-org-chain/chain-main/v8/x/nft/types"
	tieredrewardsv7testnet "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/migrations/v7testnet"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"
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

const testnetBurnAddress = "tcro1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq9dpzma"

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	app.registerV7UpgradeHandler()
	app.registerV7TestnetUpgradeHandler()
}

// registerV7UpgradeHandler registers the "v7" plan for chains that have
// not yet run it (mainnet). Testnet has already upgraded past v7 and will
// not re-run this handler.
func (app *ChainApp) registerV7UpgradeHandler() {
	planName := "v7"

	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		sdkCtx.Logger().Info("start to run module migrations...")

		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("module migrations completed!")

		if err := initInflationParams(app, sdkCtx); err != nil {
			return map[string]uint64{}, err
		}

		if err := updateMintParams(app, sdkCtx); err != nil {
			return map[string]uint64{}, err
		}

		if err := initTieredRewardsParams(app, sdkCtx); err != nil {
			return map[string]uint64{}, err
		}

		if err := initDefaultTierDefinitions(ctx, app); err != nil {
			return map[string]uint64{}, err
		}

		// Remove stale KeyDenomName("") index entry if it exists.
		// The IBC NFT transfer bug passed "" as denom name to IssueDenom,
		// which stored a name index entry for the empty string, blocking
		// all subsequent IBC NFT class creation.
		nftStore := sdkCtx.KVStore(app.keys[nfttypes.StoreKey])
		nftStore.Delete(nfttypes.KeyDenomName(""))

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
				inflationtypes.StoreKey,
				tieredrewardstypes.StoreKey,
			},
		}
		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}

// registerV7TestnetUpgradeHandler registers the "v7.1.0-testnet" plan used to
// migrate testnet from the legacy shared-pool tieredrewards model onto the
// per-position-delegator rewrite. The handler:
//
//  1. Runs module migrations.
//  2. Purges pre-rewrite tieredrewards lifecycle state (positions, secondary
//     indexes, mappings, validator events, counters, and the retired
//     ValidatorRewardRatio collection). Params and Tiers bytes survive because
//     their proto shapes are unchanged.
//  3. Unwinds the staking + bank residue still held at the tier module
//     account: every remaining delegation is undelegated, and loose bank
//     balance is swept to the testnet burn address. Matured unbondings
//     post-upgrade land back at the pool and stay trapped there — acceptable
//     on testnet.
//
// No store upgrades are wired — the inflation and tieredrewards stores
// already exist on testnet from the earlier v7 upgrade.
func (app *ChainApp) registerV7TestnetUpgradeHandler() {
	planName := "v7.1.0-testnet"

	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		sdkCtx.Logger().Info("v7-testnet: running module migrations...")
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}

		sdkCtx.Logger().Info("v7-testnet: purging pre-rewrite tieredrewards lifecycle state...")
		counts, err := tieredrewardsv7testnet.PurgeOldTieredRewardsState(sdkCtx, app.keys[tieredrewardstypes.StoreKey], tieredrewardsv7testnet.StateToPurge())
		if err != nil {
			return map[string]uint64{}, fmt.Errorf("v7-testnet tieredrewards purge: %w", err)
		}
		sdkCtx.Logger().Info("v7-testnet: tieredrewards purge completed", "deletions_per_prefix", counts)

		if err := sweepOldTierModuleResidualsTestnet(sdkCtx, app); err != nil {
			return map[string]uint64{}, fmt.Errorf("v7-testnet residual sweep: %w", err)
		}

		sdkCtx.Logger().Info("v7-testnet: upgrade completed", "plan", plan.Name, "version_map", m)
		return m, nil
	})
}

// sweepOldTierModuleResidualsTestnet unwinds pre-rewrite staking and bank
// state held by the tier module account. Called only from the v7.1.0-testnet
// handler.
//
//  1. Undelegate every delegation the pool address still holds. Funds enter
//     the staking unbonding queue and eventually mature back to the pool
//     account — at which point they stay trapped (no path out post-upgrade
//     without manual intervention). Accepted on testnet.
//  2. Sweep any loose bank balance at the pool to the testnet burn address.
func sweepOldTierModuleResidualsTestnet(ctx sdk.Context, app *ChainApp) error {
	poolAddr := app.AccountKeeper.GetModuleAddress(tieredrewardstypes.ModuleName)
	if poolAddr == nil {
		return fmt.Errorf("tieredrewards module account missing")
	}

	burnAddr, err := sdk.AccAddressFromBech32(testnetBurnAddress)
	if err != nil {
		return fmt.Errorf("parse testnet burn addr: %w", err)
	}

	stakingParams, err := app.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("get staking params: %w", err)
	}
	originalMaxEntries := stakingParams.MaxEntries
	// Temporarily lift the cap by one
	stakingParams.MaxEntries = originalMaxEntries + 1
	if err := app.StakingKeeper.SetParams(ctx, stakingParams); err != nil {
		return fmt.Errorf("bump staking MaxEntries: %w", err)
	}
	defer func() {
		stakingParams.MaxEntries = originalMaxEntries
		if restoreErr := app.StakingKeeper.SetParams(ctx, stakingParams); restoreErr != nil {
			ctx.Logger().Error("v7-testnet: failed to restore staking MaxEntries",
				"original", originalMaxEntries, "err", restoreErr)
			panic("v7-testnet: failed to restore staking MaxEntries")
		}
	}()

	delegations, err := app.StakingKeeper.GetDelegatorDelegations(ctx, poolAddr, 1000)
	if err != nil {
		return fmt.Errorf("get pool delegations: %w", err)
	}
	for _, d := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(d.ValidatorAddress)
		if err != nil {
			return fmt.Errorf("parse validator %s: %w", d.ValidatorAddress, err)
		}
		if _, _, _, err := app.StakingKeeper.Undelegate(ctx, poolAddr, valAddr, d.Shares); err != nil {
			return fmt.Errorf("v7-testnet: pool undelegate failed at validator %s (shares %s): %w",
				d.ValidatorAddress, d.Shares, err)
		}
		ctx.Logger().Info("v7-testnet: pool undelegated",
			"validator", d.ValidatorAddress, "shares", d.Shares.String())
	}

	bal := app.BankKeeper.GetAllBalances(ctx, poolAddr)
	if bal.IsZero() {
		ctx.Logger().Info("v7-testnet: pool bank balance empty, nothing to sweep")
		return nil
	}
	if err := app.BankKeeper.SendCoinsFromModuleToAccount(ctx, tieredrewardstypes.ModuleName, burnAddr, bal); err != nil {
		return fmt.Errorf("sweep pool to burn addr: %w", err)
	}
	ctx.Logger().Info("v7-testnet: pool bank balance swept to burn addr", "amount", bal.String(), "burn_addr", burnAddr.String())
	return nil
}

func initInflationParams(app *ChainApp, sdkCtx sdk.Context) error {
	sdkCtx.Logger().Info("initializing inflation params...")

	inflationParams := inflationtypes.DefaultParams()

	// update max supply to 100B * 10^8 basecro
	var ok bool
	inflationParams.MaxSupply, ok = math.NewIntFromString("10000000000000000000")
	if !ok {
		return fmt.Errorf("invalid max supply")
	}

	chainID := sdkCtx.ChainID()
	switch {
	case strings.Contains(chainID, "chaintest") || strings.Contains(chainID, "mainnet"):
		inflationParams.BurnedAddresses = []string{
			"cro1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqtcgxmv",
		}
	case strings.Contains(chainID, "testnet"):
		inflationParams.BurnedAddresses = []string{
			testnetBurnAddress,
		}
	default:
		return fmt.Errorf("unknown upgrade chain ID: %s", chainID)
	}

	inflationParams.DecayRate = math.LegacyMustNewDecFromStr("0.0680") // 6.80%

	if err := app.InflationKeeper.SetParams(sdkCtx, inflationParams); err != nil {
		return err
	}
	decayEpoch := uint64(sdkCtx.BlockHeight())
	if err := app.InflationKeeper.SetDecayEpochStart(sdkCtx, decayEpoch); err != nil {
		return err
	}

	sdkCtx.Logger().Info("inflation module initialized with params",
		"max_supply", inflationParams.MaxSupply.String(),
		"burned_addresses", inflationParams.BurnedAddresses,
		"decay_rate", inflationParams.DecayRate.String(),
		"decay_epoch_start", decayEpoch)

	return nil
}

func updateMintParams(app *ChainApp, sdkCtx sdk.Context) error {
	sdkCtx.Logger().Info("updating mint params...")
	mintParams, err := app.MintKeeper.Params.Get(sdkCtx)
	if err != nil {
		return err
	}

	mintParams.InflationMax = math.LegacyMustNewDecFromStr("0.01") // 1%
	mintParams.InflationMin = math.LegacyMustNewDecFromStr("0.01") // 1%
	// Set inflation rate change for consistency with the new decay mechanism
	mintParams.InflationRateChange = math.LegacyZeroDec()

	if err := mintParams.Validate(); err != nil {
		return err
	}

	if err := app.MintKeeper.Params.Set(sdkCtx, mintParams); err != nil {
		return err
	}

	sdkCtx.Logger().Info("mint module updated params",
		"inflation_max", mintParams.InflationMax.String(),
		"inflation_min", mintParams.InflationMin.String(),
		"inflation_rate_change", mintParams.InflationRateChange.String())

	return nil
}

func initDefaultTierDefinitions(ctx context.Context, app *ChainApp) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("initializing default tier definitions...")

	chainID := sdkCtx.ChainID()

	var tiers []tieredrewardstypes.Tier

	switch {
	case strings.Contains(chainID, "testnet"):
		minLock1CRO := math.NewInt(1).MulRaw(100_000_000)
		minute := time.Minute
		tiers = []tieredrewardstypes.Tier{
			{
				Id:            1,
				ExitDuration:  minute,
				BonusApy:      math.LegacyMustNewDecFromStr("0.02"),
				MinLockAmount: minLock1CRO,
			},
			{
				Id:            2,
				ExitDuration:  2 * minute,
				BonusApy:      math.LegacyMustNewDecFromStr("0.04"),
				MinLockAmount: minLock1CRO,
			},
			{
				Id:            3,
				ExitDuration:  4 * minute,
				BonusApy:      math.LegacyMustNewDecFromStr("0.07"),
				MinLockAmount: minLock1CRO,
			},
		}
	default:
		minLock100CRO := math.NewInt(100).MulRaw(100_000_000)
		year := time.Hour * 24 * 365
		tiers = []tieredrewardstypes.Tier{
			{
				Id:            1,
				ExitDuration:  year,
				BonusApy:      math.LegacyMustNewDecFromStr("0.02"),
				MinLockAmount: minLock100CRO,
			},
			{
				Id:            2,
				ExitDuration:  2 * year,
				BonusApy:      math.LegacyMustNewDecFromStr("0.04"),
				MinLockAmount: minLock100CRO,
			},
			{
				Id:            3,
				ExitDuration:  4 * year,
				BonusApy:      math.LegacyMustNewDecFromStr("0.07"),
				MinLockAmount: minLock100CRO,
			},
		}
	}

	for _, tier := range tiers {
		if err := app.TieredRewardsKeeper.SetTier(ctx, tier); err != nil {
			return fmt.Errorf("tieredrewards: set tier %d: %w", tier.Id, err)
		}
	}

	sdkCtx.Logger().Info("default tier definitions initialized",
		"chain_id", chainID,
		"tier_count", len(tiers))
	return nil
}

func initTieredRewardsParams(app *ChainApp, sdkCtx sdk.Context) error {
	sdkCtx.Logger().Info("initializing tiered rewards params...")
	tieredrewardsParams := tieredrewardstypes.DefaultParams()
	tieredrewardsParams.TargetBaseRewardsRate = math.LegacyMustNewDecFromStr("0.03") // 3%

	if err := app.TieredRewardsKeeper.SetParams(sdkCtx, tieredrewardsParams); err != nil {
		return err
	}

	sdkCtx.Logger().Info("tieredrewards module initialized with params",
		"target_base_rewards_rate", tieredrewardsParams.TargetBaseRewardsRate.String())

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
