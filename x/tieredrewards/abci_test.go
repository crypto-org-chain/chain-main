package tieredrewards_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
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

	params := types.NewParams(sdkmath.LegacyZeroDec())
	err := a.TieredRewardsKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	poolAddr := a.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)

	err = a.TieredRewardsKeeper.BeginBlocker(ctx)
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

	params := types.NewParams(sdkmath.LegacyNewDec(100)) // 10000%
	err := a.TieredRewardsKeeper.SetParams(ctx, params)
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
	err = a.TieredRewardsKeeper.BeginBlocker(ctx)
	require.NoError(t, err)
}

// TestBeginBlocker_TopUpFromPool verifies that the pool is drained
// by the exact shortfall amount when there's a shortfall.
func TestBeginBlocker_TopUpFromPool(t *testing.T) {
	a := testutil.Setup(false, nil)
	ctx := a.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	ctx = ctxWithVoteInfos(t, a, ctx)

	// Set the target base rewards rate to 10000% so that there is a shortfall since it is easier than increasing the total bonded tokens
	params := types.NewParams(sdkmath.LegacyNewDec(100)) // 10000%
	err := a.TieredRewardsKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	// Clear the fee collector so there's a guaranteed shortfall
	feeCollectorAddr := a.AccountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName).GetAddress()
	feeBalance := a.BankKeeper.GetAllBalances(ctx, feeCollectorAddr)
	err = a.BankKeeper.SendCoinsFromModuleToModule(ctx, authtypes.FeeCollectorName, types.RewardsPoolName, feeBalance)
	require.NoError(t, err)

	// Calculate expected shortfall (fee collector is 0, so full target is the shortfall)
	totalBonded, err := a.StakingKeeper.TotalBondedTokens(ctx)
	require.NoError(t, err)
	mintParams, err := a.MintKeeper.GetParams(ctx)
	require.NoError(t, err)
	blocksPerYear := mintParams.BlocksPerYear
	expectedShortfall := sdkmath.LegacyNewDecFromInt(totalBonded).
		Mul(params.TargetBaseRewardsRate).
		Quo(sdkmath.LegacyNewDec(int64(blocksPerYear))).
		TruncateInt()

	poolAddr := a.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	distrAddr := a.AccountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := a.BankKeeper.GetBalance(ctx, distrAddr, sdk.DefaultBondDenom)

	err = a.TieredRewardsKeeper.BeginBlocker(ctx)
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

	params := types.NewParams(sdkmath.LegacyNewDec(100)) // 10000%
	err := a.TieredRewardsKeeper.SetParams(ctx, params)
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

	poolAddr := a.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	distrAddr := a.AccountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := a.BankKeeper.GetBalance(ctx, distrAddr, sdk.DefaultBondDenom)

	err = a.TieredRewardsKeeper.BeginBlocker(ctx)
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
	params := types.NewParams(sdkmath.LegacyNewDec(100)) // 10000%
	err := a.TieredRewardsKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	// Fund the pool so we can verify it stays untouched
	poolFund := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1_000_000_000)))
	err = banktestutil.FundModuleAccount(ctx, a.BankKeeper, types.RewardsPoolName, poolFund)
	require.NoError(t, err)

	poolAddr := a.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)

	err = a.TieredRewardsKeeper.BeginBlocker(ctx)
	require.NoError(t, err)

	// Pool should be untouched
	// At 10000% rate with 1M bonded, the per-block target is 1_000_000 * 100 / 6_311_520 ≈ 15
	// the fee collector already has ~2M from mint, which more than covers the target. So the pool stays untouched.
	poolAfter := a.BankKeeper.GetBalance(ctx, poolAddr, sdk.DefaultBondDenom)
	require.Equal(t, poolBefore.Amount, poolAfter.Amount, "pool should be untouched when fee collector is sufficient")
}
