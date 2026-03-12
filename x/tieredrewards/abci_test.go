package tieredrewards_test

import (
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewards "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// ctxWithVoteInfos creates a context with VoteInfos populated from the bonded validators.
func ctxWithVoteInfos(t *testing.T, a *app.ChainApp, ctx sdk.Context) sdk.Context {
	t.Helper()
	vals, err := a.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)

	var voteInfos []abci.VoteInfo
	for _, val := range vals {
		consAddr, err := val.GetConsAddr()
		require.NoError(t, err)
		voteInfos = append(voteInfos, abci.VoteInfo{
			Validator: abci.Validator{
				Address: consAddr,
				Power:   val.GetConsensusPower(a.StakingKeeper.PowerReduction(ctx)),
			},
		})
	}
	return ctx.WithVoteInfos(voteInfos)
}

// TestBeginBlocker_ZeroRate verifies that with a zero rate, no top-up occurs.
func TestBeginBlocker_ZeroRate(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	params := types.NewParams(sdkmath.LegacyZeroDec(), []types.TierDefinition{}, []string{})
	err := a.TieredRewardsKeeper.Params.Set(ctx, params)
	require.NoError(t, err)

	poolAddr := a.TieredRewardsKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)

	err = tieredrewards.BeginBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)

	poolAfter := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	require.Equal(t, poolBefore.Amount, poolAfter.Amount)
}

// TestBeginBlocker_EmptyPool verifies that BeginBlocker skips gracefully
// when the pool has no funds.
func TestBeginBlocker_EmptyPool(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	ctx = ctxWithVoteInfos(t, a, ctx)

	params := types.NewParams(sdkmath.LegacyNewDec(100), []types.TierDefinition{}, []string{}) // 10000%
	err := a.TieredRewardsKeeper.Params.Set(ctx, params)
	require.NoError(t, err)

	// Drain the fee collector to a random address so there's a shortfall
	feeCollectorAddr := a.AccountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName).GetAddress()
	feeBalance := a.BankKeeper.GetAllBalances(ctx, feeCollectorAddr)
	if feeBalance.IsAllPositive() {
		randomAddr := sdk.AccAddress([]byte("random_addr_for_test"))
		err = a.BankKeeper.SendCoins(ctx, feeCollectorAddr, randomAddr, feeBalance)
		require.NoError(t, err)
	}

	// Pool is empty but should not panic
	err = tieredrewards.BeginBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)
}

// TestBeginBlocker_TopUpFromPool verifies that the pool is drained
// by the exact shortfall amount when there's a shortfall.
func TestBeginBlocker_TopUpFromPool(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	ctx = ctxWithVoteInfos(t, a, ctx)

	// Set the target base rewards rate to 10000% so that there is a shortfall since it is easier than increasing the total bonded tokens
	params := types.NewParams(sdkmath.LegacyNewDec(100), []types.TierDefinition{}, []string{}) // 10000%
	err := a.TieredRewardsKeeper.Params.Set(ctx, params)
	require.NoError(t, err)

	// Clear the fee collector so there's a guaranteed shortfall
	feeCollectorAddr := a.AccountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName).GetAddress()
	feeBalance := a.BankKeeper.GetAllBalances(ctx, feeCollectorAddr)
	err = a.BankKeeper.SendCoinsFromModuleToModule(ctx, authtypes.FeeCollectorName, types.RewardsPoolName, feeBalance)
	require.NoError(t, err)

	// Calculate expected shortfall (fee collector is 0, so full target is the shortfall)
	totalBonded, err := a.StakingKeeper.TotalBondedTokens(ctx)
	require.NoError(t, err)
	mintParams, err := a.TieredRewardsKeeper.GetMintParams(ctx)
	require.NoError(t, err)
	blocksPerYear := mintParams.BlocksPerYear
	expectedShortfall := sdkmath.LegacyNewDecFromInt(totalBonded).
		Mul(params.TargetBaseRewardsRate).
		Quo(sdkmath.LegacyNewDec(int64(blocksPerYear))).
		TruncateInt()

	poolAddr := a.TieredRewardsKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	distrAddr := a.AccountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := a.BankKeeper.GetBalance(ctx, distrAddr, sdk.DefaultBondDenom)

	err = tieredrewards.BeginBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)

	poolAfter := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	distrAfter := a.BankKeeper.GetBalance(ctx, distrAddr, sdk.DefaultBondDenom)
	drained := poolBefore.Amount.Sub(poolAfter.Amount)
	distrReceived := distrAfter.Amount.Sub(distrBefore.Amount)
	require.Equal(t, expectedShortfall, drained, "pool should be drained by exact shortfall amount")
	require.Equal(t, expectedShortfall, distrReceived, "distribution module should receive the exact shortfall amount")
}

// TestBeginBlocker_InsufficientPool verifies that when the pool has some
// but not enough funds, it drains what's available.
func TestBeginBlocker_InsufficientPool(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	ctx = ctxWithVoteInfos(t, a, ctx)

	params := types.NewParams(sdkmath.LegacyNewDec(100), []types.TierDefinition{}, []string{}) // 10000%
	err := a.TieredRewardsKeeper.Params.Set(ctx, params)
	require.NoError(t, err)

	// Clear the fee collector to guarantee a shortfall
	feeCollectorAddr := a.AccountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName).GetAddress()
	feeBalance := a.BankKeeper.GetAllBalances(ctx, feeCollectorAddr)
	if feeBalance.IsAllPositive() {
		randomAddr := sdk.AccAddress([]byte("random_addr_for_test"))
		err = a.BankKeeper.SendCoins(ctx, feeCollectorAddr, randomAddr, feeBalance)
		require.NoError(t, err)
	}

	// Fund pool with a small amount that is less than the shortfall
	smallAmount := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1)))
	err = banktestutil.FundModuleAccount(ctx, a.BankKeeper, types.RewardsPoolName, smallAmount)
	require.NoError(t, err)

	poolAddr := a.TieredRewardsKeeper.GetModuleAddress(types.RewardsPoolName)
	distrAddr := a.AccountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := a.BankKeeper.GetBalance(ctx, distrAddr, sdk.DefaultBondDenom)

	err = tieredrewards.BeginBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)

	// Pool should be fully drained
	poolAfter := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	distrAfter := a.BankKeeper.GetBalance(ctx, distrAddr, sdk.DefaultBondDenom)
	distrReceived := distrAfter.Amount.Sub(distrBefore.Amount)
	require.True(t, poolAfter.Amount.IsZero(), "pool should be fully drained when insufficient")
	require.Equal(t, sdkmath.NewInt(1), distrReceived, "distribution module should receive the entire pool balance")
}

// TestBeginBlocker_FeeCollectorSufficient verifies that no top-up occurs
// when the fee collector already has enough to cover the target.
func TestBeginBlocker_FeeCollectorSufficient(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	ctx = ctxWithVoteInfos(t, a, ctx)

	// Even at 10000%, the fee collector's existing balance (~2M) exceeds the
	// per-block target for 1M bonded tokens, so no top-up should occur.
	params := types.NewParams(sdkmath.LegacyNewDec(100), []types.TierDefinition{}, []string{}) // 10000%
	err := a.TieredRewardsKeeper.Params.Set(ctx, params)
	require.NoError(t, err)

	// Fund the pool so we can verify it stays untouched
	poolFund := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1_000_000_000)))
	err = banktestutil.FundModuleAccount(ctx, a.BankKeeper, types.RewardsPoolName, poolFund)
	require.NoError(t, err)

	poolAddr := a.TieredRewardsKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)

	err = tieredrewards.BeginBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)

	// Pool should be untouched
	// At 10000% rate with 1M bonded, the per-block target is 1_000_000 * 100 / 6_311_520 ≈ 15
	// the fee collector already has ~2M from mint, which more than covers the target. So the pool stays untouched.
	poolAfter := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	require.Equal(t, poolBefore.Amount, poolAfter.Amount, "pool should be untouched when fee collector is sufficient")
}

// ---------------------------------------------------------------------------
// EndBlocker tests (CRIT-2)
// ---------------------------------------------------------------------------

// setupTierParamsForABCI configures tier params on the app for ABCI-level tests.
func setupTierParamsForABCI(t *testing.T, a *app.ChainApp, ctx sdk.Context) {
	t.Helper()
	bondDenom, err := a.StakingKeeper.BondDenom(ctx)
	require.NoError(t, err)

	tiers := []types.TierDefinition{
		{
			TierId:                        1,
			ExitCommitmentDuration:        time.Hour * 24 * 365,
			ExitCommitmentDurationInYears: 1,
			BonusApy:                      sdkmath.LegacyNewDecWithPrec(4, 2),
			MinLockAmount:                 sdkmath.NewInt(1000),
		},
	}
	params := types.NewParams(sdkmath.LegacyZeroDec(), tiers, []string{bondDenom})
	err = a.TieredRewardsKeeper.Params.Set(ctx, params)
	require.NoError(t, err)
}

// TestEndBlocker_ClearsCompletedUnbonding verifies that EndBlocker clears
// the IsUnbonding flag after the unbonding completion time has passed.
func TestEndBlocker_ClearsCompletedUnbonding(t *testing.T) {
	a := testutil.Setup(false, nil)
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID}).WithBlockTime(now)

	setupTierParamsForABCI(t, a, ctx)

	user := sdk.AccAddress([]byte("test_abci_endblk_1__"))
	bondDenom, err := a.StakingKeeper.BondDenom(ctx)
	require.NoError(t, err)
	err = banktestutil.FundAccount(ctx, a.BankKeeper, user, sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(10000))))
	require.NoError(t, err)

	vals, err := a.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	val := vals[0]

	msgServer := keeper.NewMsgServerImpl(a.TieredRewardsKeeper)

	// Lock with delegation.
	resp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:     user.String(),
		TierId:    1,
		Amount:    sdk.NewCoin(bondDenom, sdkmath.NewInt(5000)),
		Validator: val.GetOperator(),
	})
	require.NoError(t, err)
	positionId := resp.PositionId

	// Trigger exit and undelegate.
	_, err = msgServer.TriggerExitFromTier(ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	require.NoError(t, err)

	_, err = msgServer.TierUndelegate(ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	require.NoError(t, err)

	// Verify position is unbonding.
	pos, err := a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.True(t, pos.IsUnbonding)
	require.False(t, pos.UnbondingCompletionTime.IsZero())

	// Run EndBlocker BEFORE completion -- should NOT clear.
	err = tieredrewards.EndBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)

	pos, err = a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.True(t, pos.IsUnbonding, "should still be unbonding before completion time")

	// Advance past unbonding completion and run EndBlocker again.
	futureCtx := ctx.WithBlockTime(pos.UnbondingCompletionTime.Add(time.Second))
	err = tieredrewards.EndBlocker(futureCtx, a.TieredRewardsKeeper)
	require.NoError(t, err)

	pos, err = a.TieredRewardsKeeper.GetPosition(futureCtx, positionId)
	require.NoError(t, err)
	require.False(t, pos.IsUnbonding, "should be cleared after completion time")
	require.Empty(t, pos.Validator)
}

// TestEndBlocker_NoPositions verifies EndBlocker is a no-op when there are no positions.
func TestEndBlocker_NoPositions(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	err := tieredrewards.EndBlocker(ctx, a.TieredRewardsKeeper)
	require.NoError(t, err)
}
