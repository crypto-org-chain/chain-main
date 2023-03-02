package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/spf13/cast"
	"golang.org/x/exp/slices"

	// tendermint imports

	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	dbm "github.com/tendermint/tm-db"

	// cosmos-sdk imports

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/store/streaming"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/group"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	groupmodule "github.com/cosmos/cosmos-sdk/x/group/module"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	// ibc-go imports

	ica "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/types"
	ibcfee "github.com/cosmos/ibc-go/v5/modules/apps/29-fee"
	ibcfeekeeper "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/keeper"
	ibcfeetypes "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/types"
	transfer "github.com/cosmos/ibc-go/v5/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v5/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v5/modules/core"
	ibcclient "github.com/cosmos/ibc-go/v5/modules/core/02-client"
	ibcclientclient "github.com/cosmos/ibc-go/v5/modules/core/02-client/client"
	ibcclienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v5/modules/core/05-port/types"
	ibchost "github.com/cosmos/ibc-go/v5/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v5/modules/core/keeper"

	// chain-main imports

	appparams "github.com/crypto-org-chain/chain-main/v4/app/params"
	"github.com/crypto-org-chain/chain-main/v4/x/chainmain"
	chainmainkeeper "github.com/crypto-org-chain/chain-main/v4/x/chainmain/keeper"
	chainmaintypes "github.com/crypto-org-chain/chain-main/v4/x/chainmain/types"
	icaauthmodule "github.com/crypto-org-chain/chain-main/v4/x/icaauth"
	icaauthmodulekeeper "github.com/crypto-org-chain/chain-main/v4/x/icaauth/keeper"
	icaauthmoduletypes "github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
	"github.com/crypto-org-chain/chain-main/v4/x/nft"
	nfttransfer "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer"
	nfttransferkeeper "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/keeper"
	nfttransfertypes "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
	nftkeeper "github.com/crypto-org-chain/chain-main/v4/x/nft/keeper"
	nfttypes "github.com/crypto-org-chain/chain-main/v4/x/nft/types"
	supply "github.com/crypto-org-chain/chain-main/v4/x/supply"
	supplykeeper "github.com/crypto-org-chain/chain-main/v4/x/supply/keeper"
	supplytypes "github.com/crypto-org-chain/chain-main/v4/x/supply/types"

	// unnamed import of statik for swagger UI support
	_ "github.com/crypto-org-chain/chain-main/v4/app/docs/statik"
)

// FIXME remove this line, dummy
const appName = "chain-maind"

var (
	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string

	// ModuleBasics defines the module BasicManager is in charge of setting up basic,
	// non-dependant module elements, such as codec registration
	// and genesis verification.
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distr.AppModuleBasic{},
		gov.NewAppModuleBasic([]govclient.ProposalHandler{
			paramsclient.ProposalHandler,
			distrclient.ProposalHandler,
			upgradeclient.LegacyProposalHandler,
			upgradeclient.LegacyCancelProposalHandler,
			ibcclientclient.UpdateClientProposalHandler,
			ibcclientclient.UpgradeProposalHandler,
		}),
		params.AppModuleBasic{},
		slashing.AppModuleBasic{},
		feegrantmodule.AppModuleBasic{},
		ibc.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		transfer.AppModuleBasic{},
		nfttransfer.AppModuleBasic{},
		authzmodule.AppModuleBasic{},
		groupmodule.AppModuleBasic{},
		vesting.AppModuleBasic{},
		ica.AppModuleBasic{},
		icaauthmodule.AppModuleBasic{},
		ibcfee.AppModuleBasic{},
		supply.AppModuleBasic{},
		chainmain.AppModuleBasic{},
		nft.AppModuleBasic{},
	)

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		ibcfeetypes.ModuleName:         nil,
		icatypes.ModuleName:            nil,
	}
)

var (
	_ App                     = (*ChainApp)(nil)
	_ servertypes.Application = (*ChainApp)(nil)
)

// ChainApp extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
type ChainApp struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry

	invCheckPeriod uint

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// keepers
	AccountKeeper       authkeeper.AccountKeeper
	BankKeeper          bankkeeper.Keeper
	CapabilityKeeper    *capabilitykeeper.Keeper
	StakingKeeper       stakingkeeper.Keeper
	SlashingKeeper      slashingkeeper.Keeper
	MintKeeper          mintkeeper.Keeper
	DistrKeeper         distrkeeper.Keeper
	GovKeeper           govkeeper.Keeper
	UpgradeKeeper       upgradekeeper.Keeper
	ParamsKeeper        paramskeeper.Keeper
	IBCKeeper           *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	IBCFeeKeeper        ibcfeekeeper.Keeper
	ICAControllerKeeper icacontrollerkeeper.Keeper
	ICAHostKeeper       icahostkeeper.Keeper
	ICAAuthKeeper       icaauthmodulekeeper.Keeper
	AuthzKeeper         authzkeeper.Keeper
	EvidenceKeeper      evidencekeeper.Keeper
	FeeGrantKeeper      feegrantkeeper.Keeper
	GroupKeeper         groupkeeper.Keeper
	TransferKeeper      ibctransferkeeper.Keeper
	NFTTransferKeeper   nfttransferkeeper.Keeper
	chainmainKeeper     chainmainkeeper.Keeper
	SupplyKeeper        supplykeeper.Keeper
	NFTKeeper           nftkeeper.Keeper

	// make scoped keepers public for test purposes
	ScopedIBCKeeper           capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper      capabilitykeeper.ScopedKeeper
	ScopedNFTTransferKeeper   capabilitykeeper.ScopedKeeper
	ScopedIBCFeeKeeper        capabilitykeeper.ScopedKeeper
	ScopedICAControllerKeeper capabilitykeeper.ScopedKeeper
	ScopedICAHostKeeper       capabilitykeeper.ScopedKeeper
	ScopedICAAuthKeeper       capabilitykeeper.ScopedKeeper

	// the module manager
	mm *module.Manager

	// simulation manager
	sm *module.SimulationManager

	// the configurator
	configurator module.Configurator
}

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, fmt.Sprintf(".%s", appName))
}

// New returns a reference to an initialized Chain.
func New(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	invCheckPeriod uint,
	encodingConfig appparams.EncodingConfig,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *ChainApp {

	// TODO: Remove cdc in favor of appCodec once all modules are migrated.
	appCodec := encodingConfig.Marshaler
	legacyAmino := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry

	bApp := baseapp.NewBaseApp(appName, logger, db, encodingConfig.TxConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)

	keys, memKeys, tkeys := StoreKeys()

	// configure state listening capabilities using AppOptions
	// we are doing nothing with the returned streamingServices and waitGroup in this case
	if _, _, err := streaming.LoadStreamingServices(bApp, appOpts, appCodec, keys); err != nil {
		tmos.Exit(err.Error())
	}

	// wire up the versiondb's `StreamingService` and `MultiStore`.
	streamers := cast.ToStringSlice(appOpts.Get("store.streamers"))
	var qms sdk.MultiStore
	if slices.Contains(streamers, "versiondb") {
		var err error
		qms, err = setupVersionDB(homePath, bApp, keys, tkeys, memKeys)
		if err != nil {
			panic(err)
		}
	}

	app := &ChainApp{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		interfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
		keys:              keys,
		tkeys:             tkeys,
		memKeys:           memKeys,
	}

	app.ParamsKeeper = initParamsKeeper(appCodec, legacyAmino, keys[paramstypes.StoreKey], tkeys[paramstypes.TStoreKey])

	// set the BaseApp's parameter store
	bApp.SetParamStore(app.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable()))

	// add capability keeper and ScopeToModule for ibc module
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(appCodec, keys[capabilitytypes.StoreKey], memKeys[capabilitytypes.MemStoreKey])
	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(ibchost.ModuleName)
	scopedTransferKeeper := app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	scopedNFTTransferKeeper := app.CapabilityKeeper.ScopeToModule(nfttransfertypes.ModuleName)
	scopedICAControllerKeeper := app.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	scopedICAHostKeeper := app.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	scopedICAAuthKeeper := app.CapabilityKeeper.ScopeToModule(icaauthmoduletypes.ModuleName)
	app.CapabilityKeeper.Seal()

	// add keepers
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec, keys[authtypes.StoreKey], app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, maccPerms, sdk.GetConfig().GetBech32AccountAddrPrefix(),
	)
	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec, keys[banktypes.StoreKey], app.AccountKeeper, app.GetSubspace(banktypes.ModuleName), app.ModuleAccountAddrs(),
	)
	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec, keys[stakingtypes.StoreKey], app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName),
	)
	app.MintKeeper = mintkeeper.NewKeeper(
		appCodec, keys[minttypes.StoreKey], app.GetSubspace(minttypes.ModuleName), &stakingKeeper,
		app.AccountKeeper, app.BankKeeper, authtypes.FeeCollectorName,
	)
	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec, keys[distrtypes.StoreKey], app.GetSubspace(distrtypes.ModuleName), app.AccountKeeper, app.BankKeeper,
		&stakingKeeper, authtypes.FeeCollectorName,
	)
	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec, keys[slashingtypes.StoreKey], &stakingKeeper, app.GetSubspace(slashingtypes.ModuleName),
	)
	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(appCodec, keys[feegrant.StoreKey], app.AccountKeeper)
	app.UpgradeKeeper = upgradekeeper.NewKeeper(skipUpgradeHeights, keys[upgradetypes.StoreKey], appCodec, homePath, app.BaseApp, authtypes.NewModuleAddress(govtypes.ModuleName).String())

	// register the staking hooks
	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
	app.StakingKeeper = *stakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(app.DistrKeeper.Hooks(), app.SlashingKeeper.Hooks()),
	)

	app.AuthzKeeper = authzkeeper.NewKeeper(keys[authzkeeper.StoreKey], appCodec, app.BaseApp.MsgServiceRouter(), app.AccountKeeper)

	groupConfig := group.DefaultConfig()
	app.GroupKeeper = groupkeeper.NewKeeper(keys[group.StoreKey], appCodec, app.MsgServiceRouter(), app.AccountKeeper, groupConfig)

	app.chainmainKeeper = *chainmainkeeper.NewKeeper(
		appCodec, keys[chainmaintypes.StoreKey], keys[chainmaintypes.MemStoreKey],
	)

	app.SupplyKeeper = supplykeeper.NewKeeper(appCodec, keys[supplytypes.StoreKey], app.BankKeeper, app.AccountKeeper)

	app.NFTKeeper = nftkeeper.NewKeeper(appCodec, keys[nfttypes.StoreKey])

	// Create IBC Keeper
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec, keys[ibchost.StoreKey], app.GetSubspace(ibchost.ModuleName), app.StakingKeeper, app.UpgradeKeeper, scopedIBCKeeper,
	)

	// register the proposal types
	govRouter := govv1beta1.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(app.ParamsKeeper)).
		AddRoute(distrtypes.RouterKey, distr.NewCommunityPoolSpendProposalHandler(app.DistrKeeper)).
		AddRoute(upgradetypes.RouterKey, upgrade.NewSoftwareUpgradeProposalHandler(app.UpgradeKeeper)).
		AddRoute(ibcclienttypes.RouterKey, ibcclient.NewClientProposalHandler(app.IBCKeeper.ClientKeeper))

	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/
	govKeeper := govkeeper.NewKeeper(
		appCodec, keys[govtypes.StoreKey], app.GetSubspace(govtypes.ModuleName), app.AccountKeeper, app.BankKeeper,
		&stakingKeeper, govRouter, app.MsgServiceRouter(), govConfig,
	)
	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(
		// register the governance hooks
		),
	)

	// IBC Fee Module keeper
	app.IBCFeeKeeper = ibcfeekeeper.NewKeeper(
		appCodec, keys[ibcfeetypes.StoreKey], app.GetSubspace(ibcfeetypes.ModuleName),
		app.IBCKeeper.ChannelKeeper, // more middlewares can be added in future
		app.IBCKeeper.ChannelKeeper,
		&app.IBCKeeper.PortKeeper, app.AccountKeeper, app.BankKeeper,
	)

	// Create Transfer Keepers
	app.TransferKeeper = ibctransferkeeper.NewKeeper(
		appCodec, keys[ibctransfertypes.StoreKey], app.GetSubspace(ibctransfertypes.ModuleName),
		app.IBCFeeKeeper, // ISC4 Wrapper: fee IBC middleware
		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		app.AccountKeeper, app.BankKeeper, scopedTransferKeeper,
	)
	transferModule := transfer.NewAppModule(app.TransferKeeper)
	feeModule := ibcfee.NewAppModule(app.IBCFeeKeeper)

	var transferStack porttypes.IBCModule
	transferStack = transfer.NewIBCModule(app.TransferKeeper)
	transferStack = ibcfee.NewIBCMiddleware(transferStack, app.IBCFeeKeeper)

	app.NFTTransferKeeper = nfttransferkeeper.NewKeeper(
		appCodec,
		keys[nfttransfertypes.StoreKey],
		app.IBCFeeKeeper,
		app.IBCKeeper.ChannelKeeper,
		&app.IBCKeeper.PortKeeper,
		app.NFTKeeper,
		app.AccountKeeper,
		scopedNFTTransferKeeper,
	)

	var nftTransferStack porttypes.IBCModule
	nftTransferStack = nfttransfer.NewIBCModule(app.NFTTransferKeeper)
	nftTransferStack = ibcfee.NewIBCMiddleware(nftTransferStack, app.IBCFeeKeeper)

	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
		appCodec, keys[icacontrollertypes.StoreKey], app.GetSubspace(icacontrollertypes.SubModuleName),
		app.IBCFeeKeeper, // ISC4 Wrapper: fee IBC middleware
		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		scopedICAControllerKeeper, app.MsgServiceRouter(),
	)
	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		appCodec, keys[icahosttypes.StoreKey], app.GetSubspace(icahosttypes.SubModuleName),
		app.IBCFeeKeeper, // ISC4 Wrapper: fee IBC middleware
		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		app.AccountKeeper, scopedICAHostKeeper, app.MsgServiceRouter(),
	)
	icaModule := ica.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper)

	app.ICAAuthKeeper = *icaauthmodulekeeper.NewKeeper(appCodec, keys[icaauthmoduletypes.StoreKey],
		keys[icaauthmoduletypes.MemStoreKey], app.GetSubspace(icaauthmoduletypes.ModuleName), app.ICAControllerKeeper,
		scopedICAAuthKeeper)

	icaAuthModule := icaauthmodule.NewAppModule(appCodec, app.ICAAuthKeeper)

	var icaControllerStack porttypes.IBCModule
	icaControllerStack = icaauthmodule.NewIBCModule(app.ICAAuthKeeper)
	icaControllerStack = icacontroller.NewIBCMiddleware(icaControllerStack, app.ICAControllerKeeper)
	icaControllerStack = ibcfee.NewIBCMiddleware(icaControllerStack, app.IBCFeeKeeper)

	var icaHostStack porttypes.IBCModule
	icaHostStack = icahost.NewIBCModule(app.ICAHostKeeper)
	icaHostStack = ibcfee.NewIBCMiddleware(icaHostStack, app.IBCFeeKeeper)

	// Create static IBC router, add transfer route, then set and seal it
	ibcRouter := porttypes.NewRouter()
	ibcRouter.AddRoute(icacontrollertypes.SubModuleName, icaControllerStack)
	ibcRouter.AddRoute(icahosttypes.SubModuleName, icaHostStack)
	ibcRouter.AddRoute(ibctransfertypes.ModuleName, transferStack)
	ibcRouter.AddRoute(nfttransfertypes.ModuleName, nftTransferStack)
	ibcRouter.AddRoute(icaauthmoduletypes.ModuleName, icaControllerStack)
	app.IBCKeeper.SetRouter(ibcRouter)

	// create evidence keeper with router
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec, keys[evidencetypes.StoreKey], &app.StakingKeeper, app.SlashingKeeper,
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	app.EvidenceKeeper = *evidenceKeeper

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper, app.BaseApp.DeliverTx,
			encodingConfig.TxConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, nil),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper),
		capability.NewAppModule(appCodec, *app.CapabilityKeeper),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, app.GovKeeper, app.AccountKeeper, app.BankKeeper),
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper, nil),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper),
		upgrade.NewAppModule(app.UpgradeKeeper),
		evidence.NewAppModule(app.EvidenceKeeper),
		ibc.NewAppModule(app.IBCKeeper),
		params.NewAppModule(app.ParamsKeeper),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		groupmodule.NewAppModule(appCodec, app.GroupKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		transferModule,
		nfttransfer.NewAppModule(app.NFTTransferKeeper),
		feeModule,
		icaModule,
		icaAuthModule,
		chainmain.NewAppModule(app.chainmainKeeper),
		supply.NewAppModule(app.SupplyKeeper),
		nft.NewAppModule(appCodec, app.NFTKeeper, app.AccountKeeper, app.BankKeeper),
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	app.mm.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibchost.ModuleName,
		ibctransfertypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		paramstypes.ModuleName,
		vestingtypes.ModuleName,
		icatypes.ModuleName,
		icaauthmoduletypes.ModuleName,
		ibcfeetypes.ModuleName,
		chainmaintypes.ModuleName,
		nfttypes.ModuleName,
		nfttransfertypes.ModuleName,
		supplytypes.ModuleName,
	)
	app.mm.SetOrderEndBlockers(
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		ibchost.ModuleName,
		ibctransfertypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		icatypes.ModuleName,
		icaauthmoduletypes.ModuleName,
		ibcfeetypes.ModuleName,
		chainmaintypes.ModuleName,
		nfttypes.ModuleName,
		nfttransfertypes.ModuleName,
		supplytypes.ModuleName,
	)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	app.mm.SetOrderInitGenesis(
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		ibchost.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		icaauthmoduletypes.ModuleName,
		ibcfeetypes.ModuleName,
		chainmaintypes.ModuleName,
		supplytypes.ModuleName,
		nfttypes.ModuleName,
		nfttransfertypes.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
	)

	app.mm.RegisterRoutes(app.Router(), app.QueryRouter(), encodingConfig.Amino)
	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.mm.RegisterServices(app.configurator)

	// add test gRPC service for testing gRPC queries in isolation
	testdata.RegisterQueryServer(app.GRPCQueryRouter(), testdata.QueryImpl{})

	// create the simulation manager and define the order of the modules for deterministic simulations
	//
	// NOTE: this is not required apps that don't use the simulator for fuzz testing
	// transactions
	overrideModules := map[string]module.AppModuleSimulation{
		// Use custom RandomGenesisAccounts so that auth module could create random EthAccounts in genesis state when genesis.json not specified
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.mm.Modules, overrideModules)

	app.sm.RegisterStoreDecoders()

	// initialize stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.MountMemoryStores(memKeys)

	// initialize BaseApp
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	anteHandler, err := NewAnteHandler(
		HandlerOptions{
			HandlerOptions: ante.HandlerOptions{
				AccountKeeper:   app.AccountKeeper,
				BankKeeper:      app.BankKeeper,
				FeegrantKeeper:  app.FeeGrantKeeper,
				SignModeHandler: encodingConfig.TxConfig.SignModeHandler(),
				SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
			},
			IBCKeeper: app.IBCKeeper,
		},
	)
	if err != nil {
		panic(err)
	}
	app.SetAnteHandler(anteHandler)
	app.SetEndBlocker(app.EndBlocker)

	// In v0.46, the SDK introduces _postHandlers_. PostHandlers are like
	// antehandlers, but are run _after_ the `runMsgs` execution. They are also
	// defined as a chain, and have the same signature as antehandlers.
	//
	// In baseapp, postHandlers are run in the same store branch as `runMsgs`,
	// meaning that both `runMsgs` and `postHandler` state will be committed if
	// both are successful, and both will be reverted if any of the two fails.
	//
	// The SDK exposes a default empty postHandlers chain.
	//
	// Please note that changing any of the anteHandler or postHandler chain is
	// likely to be a state-machine breaking change, which needs a coordinated
	// upgrade.
	app.setPostHandler()

	planName := "v4.2.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// the minimal commission rate of 5% (0.05)
		// (default is needed to be set because of SDK store migrations that set the param)
		stakingtypes.DefaultMinCommissionRate = sdk.NewDecWithPrec(5, 2)

		stakingKeeper.IterateValidators(ctx, func(index int64, val stakingtypes.ValidatorI) (stop bool) {
			if val.GetCommission().LT(stakingtypes.DefaultMinCommissionRate) {
				validator, found := stakingKeeper.GetValidator(ctx, val.GetOperator())
				if !found {
					ctx.Logger().Error("validator not found", val)
					return true
				}
				ctx.Logger().Info("update validator's commission rate to a minimal one", val)
				validator.Commission.Rate = stakingtypes.DefaultMinCommissionRate
				if validator.Commission.MaxRate.LT(stakingtypes.DefaultMinCommissionRate) {
					validator.Commission.MaxRate = stakingtypes.DefaultMinCommissionRate
				}
				stakingKeeper.SetValidator(ctx, validator)
			}
			return false
		})

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

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			tmos.Exit(err.Error())
		}

		if qms != nil {
			v1 := qms.LatestVersion()
			v2 := app.LastBlockHeight()
			if v1 > 0 && v1 != v2 {
				tmos.Exit(fmt.Sprintf("versiondb lastest version %d don't match iavl latest version %d", v1, v2))
			}
		}
	}

	app.ScopedIBCKeeper = scopedIBCKeeper
	app.ScopedTransferKeeper = scopedTransferKeeper
	app.ScopedICAControllerKeeper = scopedICAControllerKeeper
	app.ScopedICAHostKeeper = scopedICAHostKeeper
	app.ScopedICAAuthKeeper = scopedICAAuthKeeper

	return app
}

func (app *ChainApp) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		panic(err)
	}
	app.SetPostHandler(postHandler)
}

// Name returns the name of the App
func (app *ChainApp) Name() string { return app.BaseApp.Name() }

// BeginBlocker application updates every begin block
func (app *ChainApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return app.mm.BeginBlock(ctx, req)
}

// EndBlocker application updates every end block
func (app *ChainApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	return app.mm.EndBlock(ctx, req)
}

// InitChainer application update at chain initialization
func (app *ChainApp) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	var genesisState GenesisState
	if err := tmjson.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	app.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap())
	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

// LoadHeight loads a particular height
func (app *ChainApp) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *ChainApp) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// LegacyAmino returns ChainApp's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func (app *ChainApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// AppCodec returns Chain's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *ChainApp) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns Chain's InterfaceRegistry
func (app *ChainApp) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *ChainApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

// GetTKey returns the TransientStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *ChainApp) GetTKey(storeKey string) *storetypes.TransientStoreKey {
	return app.tkeys[storeKey]
}

// GetMemKey returns the MemStoreKey for the provided mem key.
//
// NOTE: This is solely used for testing purposes.
func (app *ChainApp) GetMemKey(storeKey string) *storetypes.MemoryStoreKey {
	return app.memKeys[storeKey]
}

// GetSubspace returns a param subspace for a given module name.
//
// NOTE: This is solely to be used for testing purposes.
func (app *ChainApp) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// SimulationManager implements the SimulationApp interface
func (app *ChainApp) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *ChainApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register node gRPC service for grpc-gateway.
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register grpc-gateway routes for all modules.
	ModuleBasics.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if apiConfig.Swagger {
		RegisterSwaggerAPI(clientCtx, apiSvr.Router)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *ChainApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *ChainApp) RegisterTendermintService(clientCtx client.Context) {
	tmservice.RegisterTendermintService(clientCtx, app.BaseApp.GRPCQueryRouter(), app.interfaceRegistry, app.Query)
}

func (app *ChainApp) RegisterNodeService(clientCtx client.Context) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter())
}

// RegisterSwaggerAPI registers swagger route with API Server
func RegisterSwaggerAPI(ctx client.Context, rtr *mux.Router) {
	statikFS, err := fs.NewWithNamespace("chainmain")
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	rtr.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", staticServer))
}

// GetMaccPerms returns a copy of the module account permissions
func GetMaccPerms() map[string][]string {
	dupMaccPerms := make(map[string][]string)
	for k, v := range maccPerms {
		dupMaccPerms[k] = v
	}
	return dupMaccPerms
}

// initParamsKeeper init params keeper and its subspaces
// (Amino is still needed for Ledger at the moment)
// nolint: staticcheck
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey storetypes.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govv1.ParamKeyTable())
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(ibchost.ModuleName)
	paramsKeeper.Subspace(icacontrollertypes.SubModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)
	paramsKeeper.Subspace(icaauthmoduletypes.ModuleName)

	return paramsKeeper
}

// StoreKeys returns all the store keys to register in current app
func StoreKeys() (
	map[string]*storetypes.KVStoreKey,
	map[string]*storetypes.MemoryStoreKey,
	map[string]*storetypes.TransientStoreKey,
) {
	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		minttypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		paramstypes.StoreKey,
		ibchost.StoreKey,
		upgradetypes.StoreKey,
		feegrant.StoreKey,
		evidencetypes.StoreKey,
		ibctransfertypes.StoreKey,
		icacontrollertypes.StoreKey,
		icahosttypes.StoreKey,
		capabilitytypes.StoreKey,
		authzkeeper.StoreKey,
		nfttransfertypes.StoreKey,
		group.StoreKey,
		ibcfeetypes.StoreKey,
		icaauthmoduletypes.StoreKey,
		chainmaintypes.StoreKey,
		supplytypes.StoreKey,
		nfttypes.StoreKey,
	)

	tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

	return keys, memKeys, tkeys
}
