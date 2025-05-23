package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	circuittypes "cosmossdk.io/x/circuit/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibcmigrationsv6 "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/migrations/v6"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	ibcconnectiontypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibctmmigrations "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint/migrations"
)

// CircuitSuperAdmins maps chain IDs to their super admin addresses
var CircuitSuperAdmins = map[string][]string{
	"chaintest": {
		"cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65",
		"cro1sjcrmp0ngft2n2r3r4gcva4llfj8vjdnefdg4m", // ecosystem
	},
}

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	planName := "v6.0.0"

	// Set param key table for params module migration
	for _, subspace := range app.ParamsKeeper.GetSubspaces() {
		var keyTable paramstypes.KeyTable
		switch subspace.Name() {
		case authtypes.ModuleName:
			keyTable = authtypes.ParamKeyTable() //nolint:staticcheck
		case banktypes.ModuleName:
			keyTable = banktypes.ParamKeyTable() //nolint:staticcheck
		case stakingtypes.ModuleName:
			keyTable = stakingtypes.ParamKeyTable() //nolint:staticcheck
		case minttypes.ModuleName:
			keyTable = minttypes.ParamKeyTable() //nolint:staticcheck
		case distrtypes.ModuleName:
			keyTable = distrtypes.ParamKeyTable() //nolint:staticcheck
		case slashingtypes.ModuleName:
			keyTable = slashingtypes.ParamKeyTable() //nolint:staticcheck
		case govtypes.ModuleName:
			keyTable = govv1.ParamKeyTable() //nolint:staticcheck
		case ibcexported.ModuleName:
			keyTable = ibcclienttypes.ParamKeyTable()
			keyTable.RegisterParamSet(&ibcconnectiontypes.Params{})
		case ibctransfertypes.ModuleName:
			keyTable = ibctransfertypes.ParamKeyTable()
		case icacontrollertypes.SubModuleName:
			keyTable = icacontrollertypes.ParamKeyTable()
		case icahosttypes.SubModuleName:
			keyTable = icahosttypes.ParamKeyTable()
		default:
			continue
		}
		if !subspace.HasKeyTable() {
			subspace.WithKeyTable(keyTable)
		}
	}
	baseAppLegacySS := app.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())

	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		// OPTIONAL: prune expired tendermint consensus states to save storage space
		if _, err := ibctmmigrations.PruneExpiredConsensusStates(sdkCtx, cdc, app.IBCKeeper.ClientKeeper); err != nil {
			return nil, err
		}
		// explicitly update the IBC 02-client params, adding the localhost client type
		var allowedClients []string
		ibcParamSpace := app.GetSubspace(ibcexported.ModuleName)
		ibcParamSpace.Get(sdkCtx, ibcclienttypes.KeyAllowedClients, &allowedClients)
		allowedClients = append(allowedClients, ibcexported.Localhost)
		ibcParamSpace.SetParamSet(sdkCtx, &ibcclienttypes.Params{
			AllowedClients: allowedClients,
		})
		if err := ibcmigrationsv6.MigrateICS27ChannelCapability(
			sdkCtx,
			cdc,
			app.keys["capability"],
			app.CapabilityKeeper,
			icacontrollertypes.SubModuleName,
		); err != nil {
			return nil, err
		}

		// Migrate Tendermint consensus parameters from x/params module to a dedicated x/consensus module.
		if err := baseapp.MigrateParams(sdkCtx, baseAppLegacySS, &app.ConsensusParamsKeeper.ParamsStore); err != nil {
			return nil, err
		}
		sdkCtx.Logger().Info("start to run module migrations...")

		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return map[string]uint64{}, err
		}
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
				return map[string]uint64{}, err
			}
		}

		chainID := sdkCtx.ChainID()
		if superAdmins, exists := CircuitSuperAdmins[chainID]; exists {
			for _, adminAddr := range superAdmins {
				addr, err := sdk.AccAddressFromBech32(adminAddr)
				if err != nil {
					return nil, fmt.Errorf("invalid super admin address %s: %w", adminAddr, err)
				}
				app.CircuitKeeper.Permissions.Set(sdkCtx, addr, circuittypes.Permissions{
					Level: circuittypes.Permissions_LEVEL_SUPER_ADMIN,
				})
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
			Added: []string{
				consensusparamtypes.StoreKey,
				circuittypes.StoreKey,
			},
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
