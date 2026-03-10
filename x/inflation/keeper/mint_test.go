package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"

	"cosmossdk.io/math"

	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// TestDeflationCalculationFn_NoDecay tests that deflation calculation works when decay is disabled.
func (s *KeeperSuite) TestDeflationCalculationFn_NoDecay() {
	params := types.DefaultParams()
	params.DecayRate = math.LegacyZeroDec() // Disable decay
	params.DecayStartHeight = 1000

	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	minter := minttypes.DefaultInitialMinter()
	bondedRatio := math.LegacyNewDecWithPrec(50, 2) // 0.50
	mintParams := minttypes.DefaultParams()
	baseInflation := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	// Before decay start height
	s.ctx = s.ctx.WithBlockHeight(500)
	inflation := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflation, "inflation should match default when decay is disabled")

	// After decay start height but decay disabled
	s.ctx = s.ctx.WithBlockHeight(2000)
	inflation = s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflation, "inflation should match default when decay is disabled")
}

// TestDeflationCalculationFn_WithDecay tests that deflation calculation applies decay correctly.
func (s *KeeperSuite) TestDeflationCalculationFn_WithDecay() {
	params := types.DefaultParams()
	params.DecayStartHeight = 1000
	params.DecayRate = math.LegacyNewDecWithPrec(65, 3) // 6.5% monthly decay

	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	mintParams := minttypes.DefaultParams()
	mintParams.BlocksPerYear = 6307200 // ~5 second blocks - divisible by 12 for test later

	minter := minttypes.DefaultInitialMinter()
	bondedRatio := math.LegacyNewDecWithPrec(50, 2) // 0.50
	baseInflation := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	// Before decay start height - should use base rate
	s.ctx = s.ctx.WithBlockHeight(500)
	inflationBefore := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflationBefore, "inflation should equal base before decay starts")

	// At decay start height - should still use base rate (no months elapsed)
	s.ctx = s.ctx.WithBlockHeight(1000)
	inflationAtStart := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflationAtStart, "inflation should equal base at decay start (0 months elapsed)")

	// After 1 month - should apply decay
	blocksPerMonth := mintParams.BlocksPerYear / 12
	s.ctx = s.ctx.WithBlockHeight(1000 + int64(blocksPerMonth))
	inflationAfter1Month := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	expectedDecayFactor := math.LegacyOneDec().Sub(params.DecayRate) // (1 - 0.065) = 0.935
	expectedInflation := baseInflation.Mul(expectedDecayFactor)

	// Calculate the difference and verify it's within acceptable precision
	diff := inflationAfter1Month.Sub(expectedInflation).Abs()
	// Use a tighter tolerance: 1e-12 for single month calculation
	maxTolerance := math.LegacyNewDecWithPrec(1, 12)
	s.Require().True(
		diff.LT(maxTolerance),
		"inflation after 1 month should be base * 0.935, got %s, expected %s, difference: %s (tolerance: %s)",
		inflationAfter1Month, expectedInflation, diff, maxTolerance,
	)

	// After 12 months - should apply decay^12
	s.ctx = s.ctx.WithBlockHeight(1000 + int64(blocksPerMonth*12))
	inflationAfter12Months := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	expectedDecayFactor12 := expectedDecayFactor.Power(12)
	expectedInflation12 := baseInflation.Mul(expectedDecayFactor12)

	// Calculate the difference and verify it's within acceptable precision
	diff12 := inflationAfter12Months.Sub(expectedInflation12).Abs()
	// Use tolerance: 1e-10 for 12-month calculation (Power() may accumulate small errors)
	maxTolerance12 := math.LegacyNewDecWithPrec(1, 10)
	s.Require().True(
		diff12.LT(maxTolerance12),
		"inflation after 12 months should be base * (0.935)^12, got %s, expected %s, difference: %s (tolerance: %s)",
		inflationAfter12Months, expectedInflation12, diff12, maxTolerance12,
	)
}

// TestDeflationCalculationFn_FractionalMonths tests that deflation calculation works correctly with fractional months.
func (s *KeeperSuite) TestDeflationCalculationFn_FractionalMonths() {
	params := types.DefaultParams()
	params.DecayStartHeight = 1000
	params.DecayRate = math.LegacyNewDecWithPrec(65, 3) // 6.5% monthly decay
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	mintParams := minttypes.DefaultParams()
	mintParams.BlocksPerYear = 6307200 // ~5 second blocks - divisible by 12

	minter := minttypes.DefaultInitialMinter()
	bondedRatio := math.LegacyNewDecWithPrec(50, 2) // 0.50
	baseInflation := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	blocksPerMonth := mintParams.BlocksPerYear / 12
	decayFactor := math.LegacyOneDec().Sub(params.DecayRate) // (1 - 0.065) = 0.935

	// Test 0.5 months (half a month)
	halfMonthBlocks := blocksPerMonth / 2
	s.ctx = s.ctx.WithBlockHeight(1000 + int64(halfMonthBlocks))
	inflationAfterHalfMonth := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)

	// Expected: base * decayFactor^0.5
	// Using decomposition: n=0, r=halfMonthBlocks
	// decayFactor^0.5 = (decayFactor^(1/blocksPerMonth))^halfMonthBlocks
	// Note: (x^r)^(1/m) underflows for large r; use (x^(1/m))^r instead.
	perBlockFactor, err := decayFactor.ApproxRoot(blocksPerMonth)
	s.Require().NoError(err)
	expectedDecayFactorHalfRoot := perBlockFactor.Power(halfMonthBlocks)
	expectedInflationHalf := baseInflation.Mul(expectedDecayFactorHalfRoot)

	diffHalf := inflationAfterHalfMonth.Sub(expectedInflationHalf).Abs()
	maxToleranceHalf := math.LegacyNewDecWithPrec(1, 10) // ApproxRoot may have some error
	s.Require().True(
		diffHalf.LT(maxToleranceHalf),
		"inflation after 0.5 months should be base * decayFactor^0.5, got %s, expected %s, difference: %s (tolerance: %s)",
		inflationAfterHalfMonth, expectedInflationHalf, diffHalf, maxToleranceHalf,
	)

	// Test 1.5 months
	oneAndHalfMonthBlocks := blocksPerMonth + halfMonthBlocks
	s.ctx = s.ctx.WithBlockHeight(1000 + int64(oneAndHalfMonthBlocks))
	inflationAfterOneAndHalfMonth := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)

	// Expected: base * decayFactor^1.5 = base * decayFactor^1 * decayFactor^0.5
	expectedDecayFactor1_5 := decayFactor.Power(1).Mul(expectedDecayFactorHalfRoot)
	expectedInflation1_5 := baseInflation.Mul(expectedDecayFactor1_5)

	diff1_5 := inflationAfterOneAndHalfMonth.Sub(expectedInflation1_5).Abs()
	maxTolerance1_5 := math.LegacyNewDecWithPrec(1, 10)
	s.Require().True(
		diff1_5.LT(maxTolerance1_5),
		"inflation after 1.5 months should be base * decayFactor^1.5, got %s, expected %s, difference: %s (tolerance: %s)",
		inflationAfterOneAndHalfMonth, expectedInflation1_5, diff1_5, maxTolerance1_5,
	)

	// Decay decreases inflation over time, so: inflation(1.5) < inflation(0.5) < initial base inflation
	s.ctx = s.ctx.WithBlockHeight(1000) // 0 months
	initialBaseInflation := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)

	s.Require().True(
		inflationAfterHalfMonth.LT(initialBaseInflation),
		"inflation at 0.5 months should be less than initial base inflation",
	)
	s.Require().True(
		inflationAfterOneAndHalfMonth.LT(inflationAfterHalfMonth),
		"inflation at 1.5 months should be less than inflation at 0.5 months",
	)
}

// TestDeflationCalculationFn_EdgeCases tests edge cases for fractional month calculations.
func (s *KeeperSuite) TestDeflationCalculationFn_EdgeCases() {
	params := types.DefaultParams()
	params.DecayStartHeight = 1000
	params.DecayRate = math.LegacyNewDecWithPrec(10, 2) // 10% monthly decay
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	mintParams := minttypes.DefaultParams()
	mintParams.BlocksPerYear = 6307200 // ~5 second blocks

	minter := minttypes.DefaultInitialMinter()
	bondedRatio := math.LegacyNewDecWithPrec(50, 2)
	baseInflation := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	blocksPerMonth := mintParams.BlocksPerYear / 12
	decayFactor := math.LegacyOneDec().Sub(params.DecayRate) // 0.90

	// Test at exactly decay start height (0 months elapsed)
	s.ctx = s.ctx.WithBlockHeight(1000)
	inflationAtStart := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflationAtStart, "inflation at start should equal base (0 months elapsed)")

	// Test with very small elapsed time (1 block)
	s.ctx = s.ctx.WithBlockHeight(1001)
	inflationAfter1Block := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)

	// Should have minimal decay: decayFactor^(1/blocksPerMonth)
	// Use (x^(1/m))^r to match the implementation's stable formula.
	perBlockFactorEdge, err := decayFactor.ApproxRoot(blocksPerMonth)
	s.Require().NoError(err)
	expectedInflationMinimal := baseInflation.Mul(perBlockFactorEdge.Power(1))

	diffMinimal := inflationAfter1Block.Sub(expectedInflationMinimal).Abs()
	maxToleranceMinimal := math.LegacyNewDecWithPrec(1, 10)
	s.Require().True(
		diffMinimal.LT(maxToleranceMinimal),
		"inflation after 1 block should have minimal decay, got %s, expected %s, difference: %s",
		inflationAfter1Block, expectedInflationMinimal, diffMinimal,
	)

	// Verify inflation decreases as time passes
	s.Require().True(
		inflationAfter1Block.LT(inflationAtStart),
		"inflation should decrease as time passes: %s < %s",
		inflationAfter1Block, inflationAtStart,
	)
}

// TestDeflationCalculationFn_FullDecay tests that when decayRate = 1.0 (100%),
// inflation drops to zero immediately after decayStartHeight.
func (s *KeeperSuite) TestDeflationCalculationFn_FullDecay() {
	params := types.DefaultParams()
	params.DecayStartHeight = 1000
	params.DecayRate = math.LegacyOneDec() // 100% decay
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	mintParams := minttypes.DefaultParams()
	mintParams.BlocksPerYear = 6307200

	minter := minttypes.DefaultInitialMinter()
	bondedRatio := math.LegacyNewDecWithPrec(50, 2)
	baseInflation := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	// Before decay start — should equal base inflation
	s.ctx = s.ctx.WithBlockHeight(999)
	inflation := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflation, "inflation should match base before decay start")

	// At decay start (0 blocks elapsed) — decayFactor^0 = 1, no fractional part → still base
	s.ctx = s.ctx.WithBlockHeight(1000)
	inflation = s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Equal(baseInflation, inflation, "inflation should match base at decay start (0 elapsed)")

	// 1 block after decay start — decayFactor = 0, perBlockFactor = 0^(1/m) = 0 → inflation = 0
	s.ctx = s.ctx.WithBlockHeight(1001)
	inflation = s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().True(inflation.IsZero(), "inflation should be zero with 100%% decay after 1 block, got %s", inflation)

	// Many blocks later — should still be zero
	s.ctx = s.ctx.WithBlockHeight(1000 + int64(mintParams.BlocksPerYear))
	inflation = s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)
	s.Require().True(inflation.IsZero(), "inflation should be zero with 100%% decay after 1 year, got %s", inflation)
}

// TestDeflationCalculationFn_SupplyCap tests that circulating supply doesn't exceed 100B tokens.
func (s *KeeperSuite) TestDeflationCalculationFn_SupplyCap() {
	// Set up params with decay enabled
	params := types.DefaultParams()
	params.DecayStartHeight = 1
	params.DecayRate = math.LegacyMustNewDecFromStr("0.0680") // 6.80% monthly decay
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	mintParams := minttypes.DefaultParams()
	mintParams.InflationRateChange = math.LegacyMustNewDecFromStr("0.1") // 10%
	mintParams.InflationMax = math.LegacyMustNewDecFromStr("0.01")       // 1%
	mintParams.InflationMin = math.LegacyMustNewDecFromStr("0.01")       // 1%
	mintParams.GoalBonded = math.LegacyMustNewDecFromStr("0.60")

	initialTotalSupply := math.NewInt(98_700_000_000).Mul(math.NewInt(100_000_000))                         // 98.7B * 10^8
	burnedSupply := math.NewInt(380_000_000).Mul(math.NewInt(100_000_000))                                  // 42B * 10^8
	bondedTokens := math.NewInt(17_043_807_749).Mul(math.NewInt(100_000_000))                               //  17B * 10^8
	circulatingSupply := initialTotalSupply.Sub(burnedSupply)                                               // (99B - 42B) * 10^8
	bondedRatio := math.LegacyNewDecFromInt(bondedTokens).Quo(math.LegacyNewDecFromInt(initialTotalSupply)) // ~17% bonded
	totalSupply := initialTotalSupply

	minter := minttypes.DefaultInitialMinter()
	minter.Inflation = math.LegacyNewDecWithPrec(1, 2) // Start at 1% inflation

	// Simulate many blocks (e.g., 100 years worth)
	totalBlocks := int64(mintParams.BlocksPerYear * 100) // 100 years
	blocksPerWeek := int64(mintParams.BlocksPerYear / 52)
	maxSupply := math.NewInt(100_000_000_000).Mul(math.NewInt(100_000_000)) // 100B * 10^8

	s.ctx = s.ctx.WithBlockHeight(0)

	// Track supply over time
	for block := int64(1); block <= totalBlocks; block += blocksPerWeek {
		s.ctx = s.ctx.WithBlockHeight(block)

		// Calculate inflation with decay
		inflation := s.keeper.DeflationCalculationFn()(s.ctx, minter, mintParams, bondedRatio)

		minter.Inflation = inflation

		// Update annual provisions
		minter.AnnualProvisions = minter.NextAnnualProvisions(mintParams, totalSupply)

		// Calculate block provision
		blockProvision := minter.BlockProvision(mintParams).Amount

		// Calculate weekly provision
		weeklyProvision := blockProvision.Mul(math.NewInt(blocksPerWeek))

		// Update supply
		circulatingSupply = circulatingSupply.Add(weeklyProvision)
		totalSupply = totalSupply.Add(weeklyProvision)

		// Ensure circulating supply never exceed 100B
		s.Require().True(
			circulatingSupply.LTE(maxSupply),
			"circulating supply exceeded 100B at block %d: %s > %s",
			block, circulatingSupply, maxSupply,
		)
	}

	// Max supply cap is determined by S_initial + S_initial *[ exp( baseRate / (-12 × ln(1 - decayRate)) )  -  1 ]
	// the value here corresponds to 99_045_307_761 as initial supply
	maxSupplyCap := initialTotalSupply.Add((math.NewInt(1_178_999_317).Mul(math.NewInt(100_000_000))))

	// Final check: total supply should be within 0.01% of the theoretical max supply cap.
	// The simulation uses weekly discrete steps (left Riemann sum), which slightly overestimates
	// compared to the continuous model the formula assumes. The actual chain computes block-by-block,
	// converging much closer to the theoretical cap.
	diff := totalSupply.Sub(maxSupplyCap).Abs()
	tolerance := maxSupplyCap.Quo(math.NewInt(10000)) // 0.01%
	s.Require().True(
		diff.LTE(tolerance),
		"total supply diverged from theoretical cap by more than 0.001%%: totalSupply=%s, maxSupplyCap=%s, diff=%s, tolerance=%s",
		totalSupply, maxSupplyCap, diff, tolerance,
	)
	// Final check: circulating supply should be at or below 100B
	s.Require().True(
		circulatingSupply.LTE(maxSupply),
		"circulating supply exceeded 100B: %s > %s",
		circulatingSupply, maxSupply,
	)
}

// TestDeflationCalculationFn_Cache tests that the per-block factor cache is populated,
// returns correct values, and invalidates when decayRate or blocksPerYear changes.
func (s *KeeperSuite) TestDeflationCalculationFn_Cache() {
	params := types.DefaultParams()
	params.DecayStartHeight = 1000
	params.DecayRate = math.LegacyNewDecWithPrec(65, 3) // 6.5% monthly decay
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	mintParams := minttypes.DefaultParams()
	mintParams.BlocksPerYear = 6307200 // divisible by 12

	minter := minttypes.DefaultInitialMinter()
	bondedRatio := math.LegacyNewDecWithPrec(50, 2)
	baseInflation := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	blocksPerMonth := mintParams.BlocksPerYear / 12

	// Cache should be nil before any deflation calculation
	s.Require().Nil(s.keeper.GetDecayCache(), "cache should be nil initially")

	// Before decay start: cache should remain nil (no decay path executed)
	s.ctx = s.ctx.WithBlockHeight(500)
	fn := s.keeper.DeflationCalculationFn()
	fn(s.ctx, minter, mintParams, bondedRatio)
	s.Require().Nil(s.keeper.GetDecayCache(), "cache should remain nil when before decay start height")

	// First call: should populate the cache
	// Set height past decay start so the cache path is exercised
	s.ctx = s.ctx.WithBlockHeight(int64(params.DecayStartHeight + 1000))

	result1 := fn(s.ctx, minter, mintParams, bondedRatio)
	decayFactor := math.LegacyOneDec().Sub(params.DecayRate) // 0.935
	perBlockFactor, err := decayFactor.ApproxRoot(blocksPerMonth)
	s.Require().NoError(err)

	cache1 := s.keeper.GetDecayCache()
	s.Require().NotNil(cache1, "cache1 should be populated after deflation call")
	s.Require().Equal(params.DecayRate, cache1.DecayRate, "decay rate should match")
	s.Require().Equal(mintParams.BlocksPerYear, cache1.BlocksPerYear, "blocks per year should match")
	s.Require().Equal(perBlockFactor, cache1.BlockFactor, "block factor should match")

	// Verify result1 is mathematically correct
	blocksElapsed1 := uint64(s.ctx.BlockHeight()) - params.DecayStartHeight
	n1 := blocksElapsed1 / blocksPerMonth
	r1 := blocksElapsed1 % blocksPerMonth

	intPart1 := decayFactor.Power(n1)
	fracPart1 := perBlockFactor.Power(r1)
	expectedInflation1 := baseInflation.Mul(intPart1.Mul(fracPart1))

	diff1 := result1.Sub(expectedInflation1).Abs()
	maxTolerance1 := math.LegacyNewDecWithPrec(1, 12)
	s.Require().True(
		diff1.LT(maxTolerance1),
		"result1 should match expected inflation, got %s, expected %s, diff: %s",
		result1, expectedInflation1, diff1,
	)

	// Second call with same params: should still return correct result (cache hit)
	// increase block height from first test
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1000)
	result2 := fn(s.ctx, minter, mintParams, bondedRatio)
	cache2 := s.keeper.GetDecayCache()
	s.Require().True(result2.LT(result1), "result2 decay will be larger than result1")
	s.Require().NotNil(cache2, "cache2 should be populated after deflation call")
	s.Require().Equal(cache1.DecayRate, cache2.DecayRate, "caches decay rate should match")
	s.Require().Equal(cache1.BlocksPerYear, cache2.BlocksPerYear, "caches blocks per year should match")
	s.Require().Equal(cache1.BlockFactor, cache2.BlockFactor, "caches block factor should match")

	// Verify result2 is mathematically correct
	blocksElapsed2 := uint64(s.ctx.BlockHeight()) - params.DecayStartHeight
	n2 := blocksElapsed2 / blocksPerMonth
	r2 := blocksElapsed2 % blocksPerMonth

	intPart2 := decayFactor.Power(n2)
	fracPart2 := perBlockFactor.Power(r2)
	expectedInflation2 := baseInflation.Mul(intPart2.Mul(fracPart2))

	diff2 := result2.Sub(expectedInflation2).Abs()
	maxTolerance2 := math.LegacyNewDecWithPrec(1, 12)
	s.Require().True(
		diff2.LT(maxTolerance2),
		"result2 should match expected inflation, got %s, expected %s, diff: %s",
		result2, expectedInflation2, diff2,
	)
	// Change decayRate to invalidate the cache
	params.DecayRate = math.LegacyNewDecWithPrec(10, 2) // 10% monthly decay
	err = s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	// Change blocks per year to invalidate the cache
	mintParams.BlocksPerYear = 6307200 * 2 // divisible by 12
	blocksPerMonth = mintParams.BlocksPerYear / 12

	baseInflation3 := minttypes.DefaultInflationCalculationFn(s.ctx, minter, mintParams, bondedRatio)

	// Third call: cache should be invalidated and produce a different correct result
	// Set height past decay start so the cache path is exercised
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1000)

	result3 := fn(s.ctx, minter, mintParams, bondedRatio)
	decayFactor3 := math.LegacyOneDec().Sub(params.DecayRate) // 0.90
	perBlockFactor3, err := decayFactor3.ApproxRoot(blocksPerMonth)
	s.Require().NoError(err)

	cache3 := s.keeper.GetDecayCache()
	s.Require().NotNil(cache3, "cache3 should be populated after deflation call")
	s.Require().Equal(params.DecayRate, cache3.DecayRate, "decay rate should match")
	s.Require().Equal(mintParams.BlocksPerYear, cache3.BlocksPerYear, "blocks per year should match")
	s.Require().Equal(perBlockFactor3, cache3.BlockFactor, "block factor should match")

	// Verify result3 is mathematically correct
	blocksElapsed3 := uint64(s.ctx.BlockHeight()) - params.DecayStartHeight
	n3 := blocksElapsed3 / blocksPerMonth
	r3 := blocksElapsed3 % blocksPerMonth

	intPart3 := decayFactor3.Power(n3)
	fracPart3 := perBlockFactor3.Power(r3)
	expectedInflation3 := baseInflation3.Mul(intPart3.Mul(fracPart3))

	diff3 := result3.Sub(expectedInflation3).Abs()
	maxTolerance3 := math.LegacyNewDecWithPrec(1, 12)
	s.Require().True(
		diff3.LT(maxTolerance3),
		"result3 should match expected inflation, got %s, expected %s, diff: %s",
		result3, expectedInflation3, diff3,
	)

	// Fourth call with same params: should still return correct result (cache hit)
	// increase block height from first test
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1000)
	result4 := fn(s.ctx, minter, mintParams, bondedRatio)
	cache4 := s.keeper.GetDecayCache()
	s.Require().True(result4.LT(result3), "result4 decay will be larger than result3")
	s.Require().NotNil(cache4, "cache2 should be populated after deflation call")
	s.Require().Equal(cache3.DecayRate, cache4.DecayRate, "caches decay rate should match")
	s.Require().Equal(cache3.BlocksPerYear, cache4.BlocksPerYear, "caches blocks per year should match")
	s.Require().Equal(cache3.BlockFactor, cache4.BlockFactor, "caches block factor should match")

	// Verify result4 is mathematically correct
	blocksElapsed4 := uint64(s.ctx.BlockHeight()) - params.DecayStartHeight
	n4 := blocksElapsed4 / blocksPerMonth
	r4 := blocksElapsed4 % blocksPerMonth

	intPart4 := decayFactor3.Power(n4)
	fracPart4 := perBlockFactor3.Power(r4)
	expectedInflation4 := baseInflation3.Mul(intPart4.Mul(fracPart4))

	diff4 := result4.Sub(expectedInflation4).Abs()
	maxTolerance4 := math.LegacyNewDecWithPrec(1, 12)
	s.Require().True(
		diff4.LT(maxTolerance4),
		"result4 should match expected inflation, got %s, expected %s, diff: %s",
		result4, expectedInflation4, diff4,
	)
}
