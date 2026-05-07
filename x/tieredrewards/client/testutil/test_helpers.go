package testutil

import (
	"fmt"
	"time"

	tmcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v8/app"
	apptestutil "github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardscli "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/client/cli"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	pruningtypes "cosmossdk.io/store/pruning/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	TestTierID            uint32 = 1
	TestExitDuration             = 200 * time.Millisecond
	TestUnbondingDuration        = 200 * time.Millisecond
)

func GetApp(val network.ValidatorI) servertypes.Application {
	ctx := val.GetCtx()
	appConfig := val.GetAppConfig()

	return app.New(
		ctx.Logger,
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
		baseapp.SetPruning(pruningtypes.NewPruningOptionsFromString(appConfig.Pruning)),
		baseapp.SetMinGasPrices(appConfig.MinGasPrices),
		baseapp.SetChainID(apptestutil.ChainID),
	)
}

func NewChainMainTestNetworkFixture() network.TestFixture {
	chainApp := app.New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
		baseapp.SetChainID(apptestutil.ChainID),
	)
	encCfg := chainApp.EncodingConfig()
	genesisState := chainApp.DefaultGenesis()

	var stakingGenesis stakingtypes.GenesisState
	encCfg.Marshaler.MustUnmarshalJSON(genesisState[stakingtypes.ModuleName], &stakingGenesis)
	stakingGenesis.Params.UnbondingTime = TestUnbondingDuration
	genesisState[stakingtypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&stakingGenesis)
	bondDenom := stakingGenesis.Params.BondDenom

	var tieredRewardsGenesis tieredrewardstypes.GenesisState
	encCfg.Marshaler.MustUnmarshalJSON(genesisState[tieredrewardstypes.ModuleName], &tieredRewardsGenesis)
	tieredRewardsGenesis.Tiers = []tieredrewardstypes.Tier{
		{
			Id:            TestTierID,
			ExitDuration:  TestExitDuration,
			BonusApy:      sdkmath.LegacyOneDec(),
			MinLockAmount: sdkmath.NewInt(1_000_000),
		},
	}
	genesisState[tieredrewardstypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&tieredRewardsGenesis)

	var govGenesis govv1.GenesisState
	encCfg.Marshaler.MustUnmarshalJSON(genesisState[govtypes.ModuleName], &govGenesis)
	params := govGenesis.Params
	if params == nil {
		defaultParams := govv1.DefaultParams()
		params = &defaultParams
	}

	minDeposit := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.OneInt()))
	expeditedMinDeposit := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(2)))
	maxDepositPeriod := 5 * time.Second
	votingPeriod := 5 * time.Second
	expeditedVotingPeriod := 2 * time.Second

	params.MinDeposit = minDeposit
	params.ExpeditedMinDeposit = expeditedMinDeposit
	params.MaxDepositPeriod = &maxDepositPeriod
	params.VotingPeriod = &votingPeriod
	params.ExpeditedVotingPeriod = &expeditedVotingPeriod
	govGenesis.Params = params
	genesisState[govtypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&govGenesis)

	var bankGenesis banktypes.GenesisState
	encCfg.Marshaler.MustUnmarshalJSON(genesisState[banktypes.ModuleName], &bankGenesis)
	moduleFunding := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000)))
	moduleAddr := authtypes.NewModuleAddress(tieredrewardstypes.ModuleName).String()
	bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{
		Address: moduleAddr,
		Coins:   moduleFunding,
	})
	poolFunding := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)))
	poolAddr := authtypes.NewModuleAddress(tieredrewardstypes.RewardsPoolName).String()
	bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{
		Address: poolAddr,
		Coins:   poolFunding,
	})
	bankGenesis.Supply = nil
	genesisState[banktypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&bankGenesis)

	return network.TestFixture{
		AppConstructor: GetApp,
		GenesisState:   genesisState,
		EncodingConfig: moduletestutil.TestEncodingConfig{
			InterfaceRegistry: encCfg.InterfaceRegistry,
			Codec:             encCfg.Marshaler,
			TxConfig:          encCfg.TxConfig,
			Amino:             encCfg.Amino,
		},
	}
}

func ExecQueryCmd(clientCtx client.Context, cmdArgs []string, cmdFactory func() *cobra.Command) (testutil.BufferWriter, error) {
	args := append([]string{}, cmdArgs...)
	args = append(args, fmt.Sprintf("--%s=json", tmcli.OutputFlag))

	return clitestutil.ExecTestCLICmd(clientCtx, cmdFactory(), args)
}

func ExecTxCmd(clientCtx client.Context, from string, cmdArgs []string, cmdFactory func() *cobra.Command, extraArgs ...string) (testutil.BufferWriter, error) {
	args := append([]string{}, cmdArgs...)
	args = append(args, fmt.Sprintf("--%s=%s", flags.FlagFrom, from))
	args = append(args, extraArgs...)

	return clitestutil.ExecTestCLICmd(clientCtx, cmdFactory(), args)
}

func DefaultTxArgs(bondDenom string) []string {
	return []string{
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%d", flags.FlagGas, 600_000),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(4))).String()),
	}
}

func QueryParamsExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryParams)
}

func QueryTierPositionExec(clientCtx client.Context, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{positionID}, extraArgs...), tieredrewardscli.GetCmdQueryTierPosition)
}

func QueryTierPositionsByOwnerExec(clientCtx client.Context, owner string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{owner}, extraArgs...), tieredrewardscli.GetCmdQueryTierPositionsByOwner)
}

func QueryTierPositionsByTierExec(clientCtx client.Context, tierID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{tierID}, extraArgs...), tieredrewardscli.GetCmdQueryTierPositionsByTier)
}

func QueryAllTierPositionsExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryAllTierPositions)
}

func QueryTiersExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryTiers)
}

func QueryRewardsPoolBalancesExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryRewardsPoolBalances)
}

func QueryEstimatePositionRewardsExec(clientCtx client.Context, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{positionID}, extraArgs...), tieredrewardscli.GetCmdQueryEstimatePositionRewards)
}

func QueryVotingPowerByOwnerExec(clientCtx client.Context, voter string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{voter}, extraArgs...), tieredrewardscli.GetCmdQueryVotingPowerByOwner)
}

func QueryTotalDelegatedVotingPowerExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryTotalDelegatedVotingPower)
}

func QueryRawTierPositionExec(clientCtx client.Context, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{positionID}, extraArgs...), tieredrewardscli.GetCmdQueryRawTierPosition)
}

func QueryRawTierPositionsByOwnerExec(clientCtx client.Context, owner string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{owner}, extraArgs...), tieredrewardscli.GetCmdQueryRawTierPositionsByOwner)
}

func QueryRawTierPositionsByTierExec(clientCtx client.Context, tierID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{tierID}, extraArgs...), tieredrewardscli.GetCmdQueryRawTierPositionsByTier)
}

func QueryRawAllTierPositionsExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryRawAllTierPositions)
}

func QueryValidatorDataExec(clientCtx client.Context, validator string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, append([]string{validator}, extraArgs...), tieredrewardscli.GetCmdQueryValidatorData)
}

func QueryRedelegationMappingsExec(clientCtx client.Context, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecQueryCmd(clientCtx, extraArgs, tieredrewardscli.GetCmdQueryRedelegationMappings)
}

func UpdateParamsProposalExec(clientCtx client.Context, from, params string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{params}, tieredrewardscli.GetCmdUpdateParamsProposal, extraArgs...)
}

func AddTierProposalExec(clientCtx client.Context, from, tier string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{tier}, tieredrewardscli.GetCmdAddTierProposal, extraArgs...)
}

func UpdateTierProposalExec(clientCtx client.Context, from, tier string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{tier}, tieredrewardscli.GetCmdUpdateTierProposal, extraArgs...)
}

func DeleteTierProposalExec(clientCtx client.Context, from, tierID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{tierID}, tieredrewardscli.GetCmdDeleteTierProposal, extraArgs...)
}

func LockTierExec(clientCtx client.Context, from, tierID, amount, validatorAddress string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{tierID, amount, validatorAddress}, tieredrewardscli.GetCmdLockTier, extraArgs...)
}

func CommitDelegationToTierExec(clientCtx client.Context, from, validatorAddress, amount, tierID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{validatorAddress, amount, tierID}, tieredrewardscli.GetCmdCommitDelegationToTier, extraArgs...)
}

func TierUndelegateExec(clientCtx client.Context, from, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID}, tieredrewardscli.GetCmdTierUndelegate, extraArgs...)
}

func TierRedelegateExec(clientCtx client.Context, from, positionID, dstValidator string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID, dstValidator}, tieredrewardscli.GetCmdTierRedelegate, extraArgs...)
}

func AddToTierPositionExec(clientCtx client.Context, from, positionID, amount string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID, amount}, tieredrewardscli.GetCmdAddToTierPosition, extraArgs...)
}

func TriggerExitFromTierExec(clientCtx client.Context, from, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID}, tieredrewardscli.GetCmdTriggerExitFromTier, extraArgs...)
}

func ClearPositionExec(clientCtx client.Context, from, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID}, tieredrewardscli.GetCmdClearPosition, extraArgs...)
}

func ClaimTierRewardsExec(clientCtx client.Context, from, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID}, tieredrewardscli.GetCmdClaimTierRewards, extraArgs...)
}

func WithdrawFromTierExec(clientCtx client.Context, from, positionID string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID}, tieredrewardscli.GetCmdWithdrawFromTier, extraArgs...)
}

func ExitTierWithDelegationExec(clientCtx client.Context, from, positionID, amount string, extraArgs ...string) (testutil.BufferWriter, error) {
	return ExecTxCmd(clientCtx, from, []string{positionID, amount}, tieredrewardscli.GetCmdExitTierWithDelegation, extraArgs...)
}
