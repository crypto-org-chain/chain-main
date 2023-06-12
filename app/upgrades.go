package app

import (
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/group"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ica "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts"
	icacontrollertypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/types"
	icaauthmoduletypes "github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	nfttransfertypes "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

func (app *ChainApp) RegisterUpgradeHandlers() {
	planName := "v4.2.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
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
			},
		}

		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
