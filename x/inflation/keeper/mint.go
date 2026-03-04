package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// DeflationCalculationFn returns a custom InflationCalculationFn which applies continuous exponential decay to inflation.
// Formula: inflation_rate = base_rate × (1 - monthly_decay)^months_elapsed
// where months_elapsed = blocks_elapsed / blocks_per_month (continuous decimal value).
// The base_rate is the inflation rate calculated using the default method.
// Decay starts at DecayStartHeight and uses DecayRate from params.
func (k *Keeper) DeflationCalculationFn() func(ctx context.Context, minter minttypes.Minter, params minttypes.Params, bondedRatio math.LegacyDec) math.LegacyDec {
	return func(ctx context.Context, minter minttypes.Minter, params minttypes.Params, bondedRatio math.LegacyDec) math.LegacyDec {
		inflationParams, err := k.GetParams(ctx)
		if err != nil {
			panic(fmt.Sprintf("failed to get inflation params: %s", err))
		}
		decayRate := inflationParams.DecayRate
		decayStartHeight := inflationParams.DecayStartHeight

		// Calculate base inflation rate using default method
		baseRate := minttypes.DefaultInflationCalculationFn(ctx, minter, params, bondedRatio)

		// Apply decay if enabled and we're past the start height
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		currentHeight := uint64(sdkCtx.BlockHeight())
		finalInflation := baseRate

		if decayRate.IsPositive() && currentHeight >= decayStartHeight {
			monthsInYear := uint64(12)
			blocksPerYear := params.BlocksPerYear
			blocksPerMonth := blocksPerYear / monthsInYear
			blocksElapsed := currentHeight - decayStartHeight

			if blocksPerMonth > 0 {
				// Compute months elapsed as a decimal for continuous decay
				monthsElapsed := math.LegacyNewDec(int64(blocksElapsed)).Quo(math.LegacyNewDec(int64(blocksPerMonth)))

				// Power() only accepts uint64, so decompose the exponent:
				// x^months = x^n × (x^(1/m))^r, where months = n + r/m
				// Note: we compute (x^(1/m))^r, NOT (x^r)^(1/m), because x^r underflows
				// to 0 for large r when x < 1, whereas x^(1/m) stays close to 1.
				n := uint64(monthsElapsed.TruncateInt64())
				r := blocksElapsed % blocksPerMonth

				decayFactor := math.LegacyOneDec().Sub(decayRate)
				intPart := decayFactor.Power(n)
				perBlockFactor := k.getPerBlockFactor(decayRate, blocksPerYear, blocksPerMonth) // x^(1/m)
				fracPart := perBlockFactor.Power(r)                                             // (x^(1/m))^r = x^(r/m)
				finalInflation = baseRate.Mul(intPart.Mul(fracPart))
			}
		}
		return finalInflation
	}
}

func (k *Keeper) getPerBlockFactor(decayRate math.LegacyDec, blocksPerYear, blocksPerMonth uint64) math.LegacyDec {
	cache := k.GetDecayCache()
	validCache := cache != nil && cache.DecayRate.Equal(decayRate) && cache.BlocksPerYear == blocksPerYear
	if validCache {
		return cache.BlockFactor
	}
	decayFactor := math.LegacyOneDec().Sub(decayRate)
	perBlockFactor, err := decayFactor.ApproxRoot(blocksPerMonth)
	if err != nil {
		panic(fmt.Sprintf("failed to approximate root: decayRate=%s, blocksPerYear=%d, blocksPerMonth=%d, error=%s", decayRate, blocksPerYear, blocksPerMonth, err))
	}
	updatedCache := &decayCache{
		DecayRate:     decayRate,
		BlocksPerYear: blocksPerYear,
		BlockFactor:   perBlockFactor,
	}
	k.SetDecayCache(updatedCache)
	return perBlockFactor
}
