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

func (s *KeeperSuite) ctxWithVoteInfos() sdk.Context {
	s.T().Helper()
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)

	var voteInfos []abci.VoteInfo
	for _, val := range vals {
		consAddr, err := val.GetConsAddr()
		s.Require().NoError(err)
		voteInfos = append(voteInfos, abci.VoteInfo{
			Validator: abci.Validator{
				Address: consAddr,
				Power:   val.GetConsensusPower(s.app.StakingKeeper.PowerReduction(s.ctx)),
			},
		})
	}
	return s.ctx.WithVoteInfos(voteInfos)
}

// drainFeeCollector moves all fee collector funds to a random address.
func (s *KeeperSuite) drainFeeCollector() {
	s.T().Helper()
	feeCollectorAddr := s.app.AccountKeeper.GetModuleAccount(s.ctx, authtypes.FeeCollectorName).GetAddress()
	feeBalance := s.app.BankKeeper.GetAllBalances(s.ctx, feeCollectorAddr)
	if feeBalance.IsAllPositive() {
		randomAddr := sdk.AccAddress([]byte("random_addr_for_test"))
		err := s.app.BankKeeper.SendCoins(s.ctx, feeCollectorAddr, randomAddr, feeBalance)
		s.Require().NoError(err)
	}
}

// setExtremeRate sets an extreme target rate (10000%) and returns the params.
func (s *KeeperSuite) setExtremeRate() types.Params {
	s.T().Helper()
	params := types.NewParams(sdkmath.LegacyNewDec(100))
	err := s.keeper.Params.Set(s.ctx, params)
	s.Require().NoError(err)
	return params
}

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
