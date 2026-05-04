package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// setupRedelegatingPosition creates a position with redelegation
func (s *KeeperSuite) setupRedelegatingPosition(lockAmount sdkmath.Int) (types.PositionState, uint64) {
	s.T().Helper()
	pos := s.setupNewTierPosition(lockAmount, false)

	dstValAddr, _ := s.createSecondValidator()

	msgServer := keeper.NewMsgServerImpl(s.keeper)
	res, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)
	updatedPos, err := s.keeper.LoadPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	return updatedPos, res.UnbondingId
}

// ---------------------------------------------------------------------------
// slashRedelegationPosition tests (AfterRedelegationSlashed)
// ---------------------------------------------------------------------------

// Unknown unbondingId is a no-op (non-tier delegation).
func (s *KeeperSuite) TestSlashRedelegationPosition_UnknownId() {
	s.setupTier(1)

	err := s.keeper.Hooks().AfterRedelegationSlashed(
		s.ctx, 999, sdkmath.NewInt(100), sdkmath.LegacyNewDec(50))
	s.Require().NoError(err) // no-op, no error
}

// TestSlashRedelegationPosition_ClaimsBonusRewardsUpToSlash verifies that when
// a redelegation slash fires, any bonus accrued on the destination delegation
// since the last accrual checkpoint is paid out to the position owner, and the
// position's bonus-state checkpoints (LastBonusAccrual, LastKnownBonded) are
// advanced.
func (s *KeeperSuite) TestSlashRedelegationPosition_ClaimsBonusRewardsUpToSlash() {
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	pos, unbondingId := s.setupRedelegatingPosition(lockAmount)
	owner := sdk.MustAccAddressFromBech32(pos.Owner)
	preAccrual := pos.LastBonusAccrual

	// Advance block time so bonus accrues on the destination validator.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(30 * 24 * time.Hour))

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)

	err := s.keeper.Hooks().AfterRedelegationSlashed(
		s.ctx, unbondingId, sdkmath.NewInt(100), sdkmath.LegacyNewDec(50))
	s.Require().NoError(err)

	balAfter := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom)
	s.Require().True(balAfter.Amount.GT(balBefore.Amount),
		"owner should have received bonus rewards accrued up to slash: before=%s after=%s",
		balBefore.Amount, balAfter.Amount)

	updated, err := s.keeper.LoadPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.LastBonusAccrual.After(preAccrual),
		"LastBonusAccrual should have advanced past the pre-slash checkpoint")
	s.Require().Equal(s.ctx.BlockTime(), updated.LastBonusAccrual,
		"LastBonusAccrual should advance to the slash block time")
	s.Require().True(updated.LastKnownBonded,
		"LastKnownBonded should remain true — destination validator is still bonded")
}
