package app

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/group"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ica "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts"
	icacontrollertypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v7/modules/apps/29-fee/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clientkeeper "github.com/cosmos/ibc-go/v7/modules/core/02-client/keeper"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	ibctmmigrations "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint/migrations"
	icaauthmoduletypes "github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	nfttransfertypes "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

func (app *ChainApp) RegisterUpgradeHandlers(cdc codec.BinaryCodec, clientKeeper clientkeeper.Keeper) {
	planName := "sdk47-upgrade"
	// Set param key table for params module migration
	for _, subspace := range app.ParamsKeeper.GetSubspaces() {
		var keyTable paramstypes.KeyTable
		switch subspace.Name() {
		case minttypes.ModuleName:
			keyTable = minttypes.ParamKeyTable() //nolint:staticcheck
		case ibctransfertypes.ModuleName:
			keyTable = ibctransfertypes.ParamKeyTable()
		case icacontrollertypes.SubModuleName:
			keyTable = icacontrollertypes.ParamKeyTable()
		case stakingtypes.ModuleName:
			keyTable = stakingtypes.ParamKeyTable()
		case banktypes.ModuleName:
			keyTable = banktypes.ParamKeyTable() //nolint:staticcheck
		case distrtypes.ModuleName:
			keyTable = distrtypes.ParamKeyTable() //nolint:staticcheck
		case slashingtypes.ModuleName:
			keyTable = slashingtypes.ParamKeyTable() //nolint:staticcheck
		case govtypes.ModuleName:
			keyTable = govv1.ParamKeyTable() //nolint:staticcheck
		case icahosttypes.SubModuleName:
			keyTable = icahosttypes.ParamKeyTable()
		case icaauthmoduletypes.ModuleName:
			keyTable = icaauthmoduletypes.ParamKeyTable()
		case authtypes.ModuleName:
			keyTable = authtypes.ParamKeyTable() //nolint:staticcheck
		default:
			continue
		}
		if !subspace.HasKeyTable() {
			subspace.WithKeyTable(keyTable)
		}
	}
	baseAppLegacySS := app.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// OPTIONAL: prune expired tendermint consensus states to save storage space
		if _, err := ibctmmigrations.PruneExpiredConsensusStates(ctx, cdc, clientKeeper); err != nil {
			return nil, err
		}
		// explicitly update the IBC 02-client params, adding the localhost client type
		params := clientKeeper.GetParams(ctx)
		params.AllowedClients = append(params.AllowedClients, exported.Localhost)
		clientKeeper.SetParams(ctx, params)

		// Migrate Tendermint consensus parameters from x/params module to a dedicated x/consensus module.
		baseapp.MigrateParams(ctx, baseAppLegacySS, &app.ConsensusParamsKeeper)

		// the minimal commission rate of 5% (0.05)
		// (default is needed to be set because of SDK store migrations that set the param)
		stakingtypes.DefaultMinCommissionRate = sdk.NewDecWithPrec(5, 2)

		app.StakingKeeper.IterateValidators(ctx, func(index int64, val stakingtypes.ValidatorI) (stop bool) {
			if val.GetCommission().LT(stakingtypes.DefaultMinCommissionRate) {
				validator, found := app.StakingKeeper.GetValidator(ctx, val.GetOperator())
				if !found {
					ctx.Logger().Error("validator not found", val)
					return true
				}
				ctx.Logger().Info("update validator's commission rate to a minimal one", val)
				validator.Commission.Rate = stakingtypes.DefaultMinCommissionRate
				if validator.Commission.MaxRate.LT(stakingtypes.DefaultMinCommissionRate) {
					validator.Commission.MaxRate = stakingtypes.DefaultMinCommissionRate
				}
				app.StakingKeeper.SetValidator(ctx, validator)
			}
			return false
		})

		icaModule := app.mm.Modules[icatypes.ModuleName].(ica.AppModule)

		// set the ICS27 consensus version so InitGenesis is not run
		fromVM[icatypes.ModuleName] = icaModule.ConsensusVersion()

		// create ICS27 Controller submodule params
		controllerParams := icacontrollertypes.Params{
			ControllerEnabled: false,
		}

		// create ICS27 Host submodule params
		hostParams := icahosttypes.Params{
			HostEnabled: false,
			AllowMessages: []string{
				"/cosmos.authz.v1beta1.MsgExec",
				"/cosmos.authz.v1beta1.MsgGrant",
				"/cosmos.authz.v1beta1.MsgRevoke",
				"/cosmos.bank.v1beta1.MsgSend",
				"/cosmos.bank.v1beta1.MsgMultiSend",
				"/cosmos.distribution.v1beta1.MsgSetWithdrawAddress",
				"/cosmos.distribution.v1beta1.MsgWithdrawValidatorCommission",
				"/cosmos.distribution.v1beta1.MsgFundCommunityPool",
				"/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward",
				"/cosmos.gov.v1beta1.MsgVoteWeighted",
				"/cosmos.gov.v1beta1.MsgSubmitProposal",
				"/cosmos.gov.v1beta1.MsgDeposit",
				"/cosmos.gov.v1beta1.MsgVote",
				"/cosmos.staking.v1beta1.MsgCreateValidator",
				"/cosmos.staking.v1beta1.MsgEditValidator",
				"/cosmos.staking.v1beta1.MsgDelegate",
				"/cosmos.staking.v1beta1.MsgUndelegate",
				"/cosmos.staking.v1beta1.MsgBeginRedelegate",
				"/cosmos.staking.v1beta1.MsgCancelUnbondingDelegation",
				"/cosmos.slashing.v1beta1.MsgUnjail",
				"/ibc.applications.transfer.v1.MsgTransfer",
				"/chainmain.nft_transfer.v1.MsgTransfer",
				"/chainmain.nft.v1.MsgBurnNFT",
				"/chainmain.nft.v1.MsgEditNFT",
				"/chainmain.nft.v1.MsgIssueDenom",
				"/chainmain.nft.v1.MsgMintNFT",
				"/chainmain.nft.v1.MsgTransferNFT",
			},
		}

		ctx.Logger().Info("start to init interchain account module...")

		// initialize ICS27 module
		icaModule.InitModule(ctx, controllerParams, hostParams)

		ctx.Logger().Info("start to run module migrations...")

		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	// testnets need to do a coordinated upgrade to keep in sync with current mainnet version
	testnetPlanName := "v4.2.7-testnet"
	app.UpgradeKeeper.SetUpgradeHandler(testnetPlanName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}

	if upgradeInfo.Name == planName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{
				group.ModuleName,
				icacontrollertypes.StoreKey,
				icahosttypes.StoreKey,
				icaauthmoduletypes.StoreKey,
				ibcfeetypes.StoreKey,
				nfttransfertypes.StoreKey,
				consensusparamtypes.StoreKey,
				crisistypes.StoreKey,
			},
		}

		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
