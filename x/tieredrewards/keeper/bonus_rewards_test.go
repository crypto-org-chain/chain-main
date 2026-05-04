package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ---------------------------------------------------------------------------
// Bonus rewards -- ComputeSegmentBonus
// ---------------------------------------------------------------------------

// TestComputeSegmentBonus_BondedValidator verifies that ComputeSegmentBonus
// computes a positive bonus when the position is bonded for 30 days.
func (s *KeeperSuite) TestComputeSegmentBonus_BondedValidator() {
	pos := s.setupNewTierPosition(sdkmath.NewInt(10000), false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	blockTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	tokensPerShare := val.TokensFromShares(sdkmath.LegacyOneDec())
	expectedBonus := s.keeper.ComputeSegmentBonus(pos, tier, pos.LastBonusAccrual, blockTime, tokensPerShare)

	s.Require().True(expectedBonus.IsPositive(),
		"bonus should be positive for a bonded validator with 30 days accrual")
}

// TestComputeSegmentBonus_ZeroAmount verifies that bonus is zero when the
// position has zero delegated shares (e.g. after 100% slash on redelegation).
func (s *KeeperSuite) TestComputeSegmentBonus_ZeroAmount() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, uint32(1))
	s.Require().NoError(err)

	blockTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	tokensPerShare := val.TokensFromShares(sdkmath.LegacyOneDec())

	// Position with zero shares (100% slash on redelegation).
	pos := types.PositionState{
		Position: types.Position{
			LastBonusAccrual: s.ctx.BlockTime(),
		},
		Delegation: &stakingtypes.Delegation{Shares: sdkmath.LegacyZeroDec()},
	}
	bonus := s.keeper.ComputeSegmentBonus(pos, tier, pos.LastBonusAccrual, blockTime, tokensPerShare)
	s.Require().True(bonus.IsZero(),
		"bonus should be zero for undelegated position with zero shares")
}

// TestComputeSegmentBonus_SharesWorthless verifies that bonus is zero when
// the position has non-zero delegated shares but the validator has been
// slashed to zero tokens, making TokensFromShares return zero.
func (s *KeeperSuite) TestComputeSegmentBonus_SharesWorthless() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	pos := s.setupNewTierPosition(lockAmount, false)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	s.Require().True(pos.Delegation.Shares.IsPositive(), "should have shares")

	// Slash validator to zero -- shares remain but are now worthless.
	s.slashValidatorDirect(valAddr, sdkmath.LegacyOneDec())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(val.GetTokens().IsZero(),
		"validator tokens should be zero after 100% slash")

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	blockTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)

	// TokensFromShares returns zero because validator has no tokens.
	tokens := val.TokensFromShares(pos.Delegation.Shares)
	s.Require().True(tokens.IsZero(),
		"tokens from shares should be zero for slashed validator")

	tokensPerShare := val.TokensFromShares(sdkmath.LegacyOneDec())
	bonus := s.keeper.ComputeSegmentBonus(pos, tier, pos.LastBonusAccrual, blockTime, tokensPerShare)
	s.Require().True(bonus.IsZero(),
		"bonus should be zero when shares are worthless (validator slashed to zero)")
}

// TestComputeSegmentBonus_ZeroSegmentDuration verifies that bonus is zero
// when segmentStart equals segmentEnd (zero duration).
func (s *KeeperSuite) TestComputeSegmentBonus_ZeroSegmentDuration() {
	s.setupTier(1)
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, uint32(1))
	s.Require().NoError(err)

	tokensPerShare := val.TokensFromShares(sdkmath.LegacyOneDec())

	pos := types.PositionState{
		Position: types.Position{
			LastBonusAccrual: s.ctx.BlockTime(),
		},
		Delegation: &stakingtypes.Delegation{
			ValidatorAddress: valAddr.String(),
			Shares:           sdkmath.LegacyNewDec(10000),
		},
	}
	// segmentStart == segmentEnd -> zero duration -> zero bonus.
	bonus := s.keeper.ComputeSegmentBonus(pos, tier, s.ctx.BlockTime(), s.ctx.BlockTime(), tokensPerShare)
	s.Require().True(bonus.IsZero(),
		"bonus should be zero when segment duration is zero")
}

// TestComputeSegmentBonus_CappedAtExitUnlockAt verifies that when segmentEnd
// exceeds ExitUnlockAt, the bonus is capped at ExitUnlockAt — not the full
// segment duration. This prevents bonus accrual after the exit lock expires.
func (s *KeeperSuite) TestComputeSegmentBonus_CappedAtExitUnlockAt() {
	s.setupTier(1)
	tier, _ := s.keeper.Tiers.Get(s.ctx, 1)
	tier.BonusApy = sdkmath.LegacyNewDecWithPrec(4, 2) // 4%

	now := s.ctx.BlockTime()
	tokensPerShare := sdkmath.LegacyNewDec(1)
	exitUnlockAt := now.Add(10 * 24 * time.Hour) // 10 days from now

	pos := types.PositionState{
		Position: types.Position{
			ExitUnlockAt: exitUnlockAt,
		},
		Delegation: &stakingtypes.Delegation{Shares: sdkmath.LegacyNewDec(1000)},
	}

	// Segment that extends 365 days past ExitUnlockAt.
	segmentEnd := exitUnlockAt.Add(365 * 24 * time.Hour)
	bonusCapped := s.keeper.ComputeSegmentBonus(pos, tier, now, segmentEnd, tokensPerShare)

	// Compare against exact 10-day bonus (capped at ExitUnlockAt).
	bonusExact := s.keeper.ComputeSegmentBonus(pos, tier, now, exitUnlockAt, tokensPerShare)

	s.Require().Equal(bonusExact.String(), bonusCapped.String(),
		"bonus should be capped at ExitUnlockAt regardless of segment end; capped=%s, exact=%s",
		bonusCapped, bonusExact)
	s.Require().True(bonusCapped.IsPositive(), "capped bonus should still be positive")

	// Segment that starts AFTER ExitUnlockAt should yield zero.
	bonusAfter := s.keeper.ComputeSegmentBonus(pos, tier, exitUnlockAt, segmentEnd, tokensPerShare)
	s.Require().True(bonusAfter.IsZero(),
		"segment starting at ExitUnlockAt should yield zero bonus")
}

// TestComputeSegmentBonus_CorrectAmount verifies the exact formula:
//
//	bonus = shares × tokensPerShare × bonusApy × durationSeconds / SecondsPerYear
//
// Uses known values so the expected result can be computed by hand.
//
// Example:
//
//	shares         = 1000
//	tokensPerShare = 2.0  (1 share = 2 tokens)
//	bonusApy       = 0.04 (4%)
//	duration       = 365.25 days (exactly 1 year = 31557600 seconds)
//
//	bonus = 1000 × 2.0 × 0.04 × 31557600 / 31557600
//	      = 1000 × 2.0 × 0.04
//	      = 80
func (s *KeeperSuite) TestComputeSegmentBonus_CorrectAmount() {
	s.setupTier(1)
	tier, err := s.keeper.Tiers.Get(s.ctx, 1)
	s.Require().NoError(err)

	// Override tier APY to a known value: 4%.
	tier.BonusApy = sdkmath.LegacyNewDecWithPrec(4, 2) // 0.04

	now := s.ctx.BlockTime()
	oneYear := time.Duration(types.SecondsPerYear) * time.Second

	shares := sdkmath.LegacyNewDec(1000)
	tokensPerShare := sdkmath.LegacyNewDec(2) // 1 share = 2 tokens

	pos := types.PositionState{
		Delegation: &stakingtypes.Delegation{Shares: shares},
	}

	bonus := s.keeper.ComputeSegmentBonus(pos, tier, now, now.Add(oneYear), tokensPerShare)

	// Expected: 1000 shares × 2 tokens/share × 0.04 apy × 1 year = 80
	s.Require().Equal(sdkmath.NewInt(80).String(), bonus.String(),
		"bonus for 1000 shares × 2 tokensPerShare × 4%% APY × 1 year should be exactly 80")

	// Half year should yield half.
	halfYear := oneYear / 2
	bonusHalf := s.keeper.ComputeSegmentBonus(pos, tier, now, now.Add(halfYear), tokensPerShare)
	s.Require().Equal(sdkmath.NewInt(40).String(), bonusHalf.String(),
		"bonus for half a year should be exactly 40")

	// Double the shares should double the bonus.
	pos.Delegation.Shares = sdkmath.LegacyNewDec(2000)
	bonusDoubleShares := s.keeper.ComputeSegmentBonus(pos, tier, now, now.Add(oneYear), tokensPerShare)
	s.Require().Equal(sdkmath.NewInt(160).String(), bonusDoubleShares.String(),
		"bonus for 2000 shares should be double: 160")

	// Double the rate should double the bonus.
	pos.Delegation.Shares = sdkmath.LegacyNewDec(1000)
	doubleRate := sdkmath.LegacyNewDec(4) // 1 share = 4 tokens
	bonusDoubleRate := s.keeper.ComputeSegmentBonus(pos, tier, now, now.Add(oneYear), doubleRate)
	s.Require().Equal(sdkmath.NewInt(160).String(), bonusDoubleRate.String(),
		"bonus with doubled rate should be 160")

	// Zero APY should yield zero.
	tier.BonusApy = sdkmath.LegacyZeroDec()
	bonusZeroApy := s.keeper.ComputeSegmentBonus(pos, tier, now, now.Add(oneYear), tokensPerShare)
	s.Require().True(bonusZeroApy.IsZero(), "zero APY should yield zero bonus")

	// Zero shares should yield zero.
	tier.BonusApy = sdkmath.LegacyNewDecWithPrec(4, 2)
	pos.Delegation.Shares = sdkmath.LegacyZeroDec()
	bonusZeroShares := s.keeper.ComputeSegmentBonus(pos, tier, now, now.Add(oneYear), tokensPerShare)
	s.Require().True(bonusZeroShares.IsZero(), "zero shares should yield zero bonus")
}
