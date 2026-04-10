package keeper_test

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// TestBeginBlocker_ZeroRate verifies that with a zero rate, no top-up occurs.
func (s *KeeperSuite) TestBeginBlocker_ZeroRate() {
	params := types.NewParams(sdkmath.LegacyZeroDec())
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)

	err = s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)

	poolAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	s.Require().Equal(poolBefore.Amount, poolAfter.Amount)
}

// TestBeginBlocker_EmptyPool verifies that BeginBlocker skips gracefully
// when the pool has no funds.
func (s *KeeperSuite) TestBeginBlocker_EmptyPool() {
	s.ctx = s.ctxWithVoteInfos()
	s.setExtremeRate()
	s.drainFeeCollector()

	err := s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)
}

// TestBeginBlocker_TopUpFromPool verifies that the pool is drained
// by the exact shortfall amount when there's a shortfall.
func (s *KeeperSuite) TestBeginBlocker_TopUpFromPool() {
	s.ctx = s.ctxWithVoteInfos()
	params := s.setExtremeRate()

	// Clear the fee collector so there's a guaranteed shortfall
	feeCollectorAddr := s.app.AccountKeeper.GetModuleAccount(s.ctx, authtypes.FeeCollectorName).GetAddress()
	feeBalance := s.app.BankKeeper.GetAllBalances(s.ctx, feeCollectorAddr)
	err := s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, authtypes.FeeCollectorName, types.RewardsPoolName, feeBalance)
	s.Require().NoError(err)

	// Calculate expected shortfall (fee collector is 0, so full target is the shortfall)
	totalBonded, err := s.app.StakingKeeper.TotalBondedTokens(s.ctx)
	s.Require().NoError(err)
	mintParams, err := s.app.MintKeeper.GetParams(s.ctx)
	s.Require().NoError(err)
	expectedShortfall := sdkmath.LegacyNewDecFromInt(totalBonded).
		Mul(params.TargetBaseRewardsRate).
		Quo(sdkmath.LegacyNewDec(int64(mintParams.BlocksPerYear))).
		TruncateInt()

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	s.Require().True(poolBefore.Amount.GTE(expectedShortfall),
		"test assumption: pool must have enough to cover shortfall; pool=%s shortfall=%s",
		poolBefore.Amount, expectedShortfall)
	distrAddr := s.app.AccountKeeper.GetModuleAccount(s.ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := s.app.BankKeeper.GetBalance(s.ctx, distrAddr, sdk.DefaultBondDenom)

	err = s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)

	poolAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	distrAfter := s.app.BankKeeper.GetBalance(s.ctx, distrAddr, sdk.DefaultBondDenom)
	drained := poolBefore.Amount.Sub(poolAfter.Amount)
	distrReceived := distrAfter.Amount.Sub(distrBefore.Amount)
	s.Require().Equal(expectedShortfall, drained, "pool should be drained by exact shortfall amount")
	s.Require().Equal(expectedShortfall, distrReceived, "distribution module should receive the exact shortfall amount")
}

// TestBeginBlocker_InsufficientPool verifies that when the pool has some
// but not enough funds, it drains what's available.
func (s *KeeperSuite) TestBeginBlocker_InsufficientPool() {
	s.ctx = s.ctxWithVoteInfos()
	s.setExtremeRate()
	s.drainFeeCollector()

	// Fund pool with a small amount that is less than the shortfall
	smallAmount := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1)))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, smallAmount)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	distrAddr := s.app.AccountKeeper.GetModuleAccount(s.ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := s.app.BankKeeper.GetBalance(s.ctx, distrAddr, sdk.DefaultBondDenom)

	err = s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)

	poolAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	distrAfter := s.app.BankKeeper.GetBalance(s.ctx, distrAddr, sdk.DefaultBondDenom)
	distrReceived := distrAfter.Amount.Sub(distrBefore.Amount)
	s.Require().True(poolAfter.Amount.IsZero(), "pool should be fully drained when insufficient")
	s.Require().Equal(sdkmath.NewInt(1), distrReceived, "distribution module should receive the entire pool balance")
}

// TestBeginBlocker_FeeCollectorSufficient verifies that no top-up occurs
// when the fee collector already has enough to cover the target.
func (s *KeeperSuite) TestBeginBlocker_FeeCollectorSufficient() {
	s.ctx = s.ctxWithVoteInfos()
	s.setExtremeRate()

	// Fund the pool so we can verify it stays untouched
	poolFund := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1_000_000_000)))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, poolFund)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)

	err = s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)

	poolAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	s.Require().Equal(poolBefore.Amount, poolAfter.Amount, "pool should be untouched when fee collector is sufficient")
}

// TestBeginBlocker_BlocksPerYearZero verifies that BeginBlocker returns nil without
// panicking when blocksPerYear is zero.
func (s *KeeperSuite) TestBeginBlocker_BlocksPerYearZero() {
	mintParams, err := s.app.MintKeeper.Params.Get(s.ctx)
	s.Require().NoError(err)
	mintParams.BlocksPerYear = 0
	err = s.app.MintKeeper.Params.Set(s.ctx, mintParams)
	s.Require().NoError(err)

	params := types.NewParams(sdkmath.LegacyNewDecWithPrec(3, 2))
	err = s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	err = s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)
}

// TestBeginBlocker_DustGoesToLastValidator verifies that power-fraction truncation
// dust is allocated to the last validator so the full top-up amount is distributed
// without leaving untracked coins in the distribution module.
func (s *KeeperSuite) TestBeginBlocker_DustGoesToLastValidator() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)

	firstConsAddr, err := vals[0].GetConsAddr()
	s.Require().NoError(err)
	secondValAddr, _ := s.createSecondValidator()
	secondVal, err := s.app.StakingKeeper.GetValidator(s.ctx, secondValAddr)
	s.Require().NoError(err)
	secondConsAddr, err := secondVal.GetConsAddr()
	s.Require().NoError(err)

	// Simulate two validators with power 1 and 6 (total 7).
	// Validator 0 gets 1/7 of rewards — this truncates and produces dust.
	// Validator 1 (last) receives the remainder including the dust.
	totalPower := int64(7)
	s.ctx = s.ctx.WithVoteInfos([]abci.VoteInfo{
		{Validator: abci.Validator{Address: firstConsAddr, Power: 1}},
		{Validator: abci.Validator{Address: secondConsAddr, Power: 6}},
	})

	s.setExtremeRate()
	s.drainFeeCollector()

	poolFund := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1_000_000_000)))
	err = banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, poolFund)
	s.Require().NoError(err)

	// Compute the shortfall (= topUp amount) and verify it's not divisible by total power, so dust will exist
	totalBonded, err := s.app.StakingKeeper.TotalBondedTokens(s.ctx)
	s.Require().NoError(err)
	mintParams, err := s.app.MintKeeper.GetParams(s.ctx)
	s.Require().NoError(err)
	params, err := s.keeper.Params.Get(s.ctx)
	s.Require().NoError(err)
	topUpAmount := sdkmath.LegacyNewDecFromInt(totalBonded).
		Mul(params.TargetBaseRewardsRate).
		Quo(sdkmath.LegacyNewDec(int64(mintParams.BlocksPerYear))).
		TruncateInt()
	s.Require().False(topUpAmount.ModRaw(totalPower).IsZero(),
		"test assumption: topUp amount (%s) must not be divisible by %d to produce dust", topUpAmount, totalPower)

	firstValAddr, err := sdk.ValAddressFromBech32(vals[0].OperatorAddress)
	s.Require().NoError(err)
	firstOutstandingBefore, err := s.app.DistrKeeper.GetValidatorOutstandingRewardsCoins(s.ctx, firstValAddr)
	s.Require().NoError(err)
	secondOutstandingBefore, err := s.app.DistrKeeper.GetValidatorOutstandingRewardsCoins(s.ctx, secondValAddr)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	distrAddr := s.app.AccountKeeper.GetModuleAccount(s.ctx, distrtypes.ModuleName).GetAddress()
	distrBefore := s.app.BankKeeper.GetBalance(s.ctx, distrAddr, sdk.DefaultBondDenom)

	err = s.keeper.BeginBlocker(s.ctx)
	s.Require().NoError(err)

	poolAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, sdk.DefaultBondDenom)
	distrAfter := s.app.BankKeeper.GetBalance(s.ctx, distrAddr, sdk.DefaultBondDenom)

	drained := poolBefore.Amount.Sub(poolAfter.Amount)
	distrReceived := distrAfter.Amount.Sub(distrBefore.Amount)

	s.Require().True(drained.IsPositive(), "pool should be drained")
	s.Require().Equal(drained, distrReceived,
		"distribution module must receive the full top-up amount (no dust lost); drained=%s received=%s",
		drained, distrReceived)

	firstOutstandingAfter, err := s.app.DistrKeeper.GetValidatorOutstandingRewardsCoins(s.ctx, firstValAddr)
	s.Require().NoError(err)
	secondOutstandingAfter, err := s.app.DistrKeeper.GetValidatorOutstandingRewardsCoins(s.ctx, secondValAddr)
	s.Require().NoError(err)

	firstAllocated := firstOutstandingAfter.Sub(firstOutstandingBefore)
	secondAllocated := secondOutstandingAfter.Sub(secondOutstandingBefore)

	topUpDec := sdk.NewDecCoins(sdk.NewDecCoin(sdk.DefaultBondDenom, drained))
	firstExpected := topUpDec.MulDecTruncate(sdkmath.LegacyNewDec(1).QuoTruncate(sdkmath.LegacyNewDec(totalPower)))
	secondExpected := topUpDec.Sub(firstExpected)

	s.Require().True(firstAllocated.Equal(firstExpected),
		"first validator should receive truncated 1/%d share; got=%s want=%s",
		totalPower, firstAllocated, firstExpected,
	)
	s.Require().True(secondAllocated.Equal(secondExpected),
		"last validator should receive the full remainder; got=%s want=%s",
		secondAllocated, secondExpected,
	)
}
