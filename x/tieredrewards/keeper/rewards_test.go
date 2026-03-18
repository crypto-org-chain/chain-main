package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// ---------------------------------------------------------------------------
// Bonus rewards: validator status tracking
// ---------------------------------------------------------------------------

// setupPositionForBonusTest creates a funded, delegated position and funds
// the rewards pool so that bonus can actually be paid out.
func (s *KeeperSuite) setupPositionForBonusTest() (sdk.AccAddress, sdk.ValAddress, types.Position) {
	s.T().Helper()
	_, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	addr := sdk.AccAddress([]byte("bonus_test_addr_____"))
	lockAmount := sdkmath.NewInt(10000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            addr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	return addr, valAddr, positions[0]
}

// jailAndUnbondValidator jails a validator and runs ApplyAndReturnValidatorSetUpdates
// so the validator actually transitions to unbonding (which fires the hooks).
func (s *KeeperSuite) jailAndUnbondValidator(valAddr sdk.ValAddress) {
	s.T().Helper()
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Jail(s.ctx, consAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)
}

// Claiming bonus while validator is bonded should yield positive bonus.
func (s *KeeperSuite) TestClaimBonusRewards_BondedValidator() {
	addr, _, pos := s.setupPositionForBonusTest()

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, tier, false)
	s.Require().NoError(err)

	s.Require().False(bonus.IsZero(),
		"bonus should be positive for a bonded validator; owner=%s", addr.String())
}

// AfterValidatorBeginUnbonding settles the final bonus (forceAccrue) and
// advances LastBonusAccrual to block time. Subsequent claims see the
// validator as unbonding and calculateBonus returns zero.
func (s *KeeperSuite) TestAfterValidatorBeginUnbonding_SettlesFinalBonus() {
	_, valAddr, pos := s.setupPositionForBonusTest()

	unbondTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(unbondTime)

	s.jailAndUnbondValidator(valAddr)

	// LastBonusAccrual should be advanced to the block time (not zeroed).
	updated, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(unbondTime, updated.LastBonusAccrual,
		"LastBonusAccrual should be advanced to block time after unbonding hook")
}

// MsgClaimTierRewards returns zero bonus when the validator is not bonded.
func (s *KeeperSuite) TestClaimTierRewards_UnbondingValidator_ZeroBonus() {
	addr, valAddr, pos := s.setupPositionForBonusTest()

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Jail + apply → unbonding (hook settles final bonus).
	s.jailAndUnbondValidator(valAddr)

	// Advance time further; the validator is still unbonding.
	claimTime := s.ctx.BlockTime().Add(30 * 24 * time.Hour)
	s.ctx = s.ctx.WithBlockTime(claimTime)

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	resp, err := msgServer.ClaimTierRewards(s.ctx, &types.MsgClaimTierRewards{
		Owner:      addr.String(),
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	s.Require().True(resp.BonusRewards.IsZero(),
		"bonus should be zero for an unbonding validator; got %s", resp.BonusRewards)

	// LastBonusAccrual should advance to current block time.
	updated, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().Equal(claimTime, updated.LastBonusAccrual,
		"LastBonusAccrual should advance to block time even when bonus is zero")
}

// After the validator re-bonds, bonus accrual should resume from the new bonded time.
func (s *KeeperSuite) TestBonusAccrual_ResumesAfterRebond() {
	_, valAddr, pos := s.setupPositionForBonusTest()

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Jail + apply → unbonding (hook settles final bonus).
	s.jailAndUnbondValidator(valAddr)

	// Verify LastBonusAccrual was advanced (not zeroed).
	updated, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(updated.LastBonusAccrual.IsZero())

	// Unjail and apply to re-bond.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Unjail(s.ctx, consAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	if val.IsBonded() {
		// AfterValidatorBonded should have reset LastBonusAccrual.
		updated, err = s.keeper.Positions.Get(s.ctx, pos.Id)
		s.Require().NoError(err)
		s.Require().False(updated.LastBonusAccrual.IsZero(),
			"LastBonusAccrual should be reset after validator re-bonds")
	}
}

// calculateBonus returns zero when the validator is not bonded.
func (s *KeeperSuite) TestCalculateBonus_UnbondedValidator_ReturnsZero() {
	_, valAddr, pos := s.setupPositionForBonusTest()

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	s.jailAndUnbondValidator(valAddr)

	// Advance time so there would be a non-zero bonus if the validator were bonded.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	// Re-read position (hook advanced LastBonusAccrual).
	pos, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// forceAccrue=false → calculateBonus → returns zero (validator not bonded).
	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, tier, false)
	s.Require().NoError(err)
	s.Require().True(bonus.IsZero(),
		"bonus should be zero when validator is not bonded; got %s", bonus)
}

// forceAccrue=true still yields bonus even when the validator is not bonded.
func (s *KeeperSuite) TestClaimBonusRewards_ForceAccrue() {
	_, valAddr, pos := s.setupPositionForBonusTest()

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	s.jailAndUnbondValidator(valAddr)

	// Advance time.
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	pos, err := s.keeper.Positions.Get(s.ctx, pos.Id)
	s.Require().NoError(err)

	tier, err := s.keeper.Tiers.Get(s.ctx, pos.TierId)
	s.Require().NoError(err)

	// forceAccrue=true → calculateBonusRaw → ignores validator status.
	bonus, err := s.keeper.ClaimBonusRewards(s.ctx, &pos, tier, true)
	s.Require().NoError(err)
	s.Require().False(bonus.IsZero(),
		"forceAccrue=true should yield bonus even for an unbonded validator")
}
