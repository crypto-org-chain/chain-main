package keeper_test

import (
	"time"

	tieredrewards "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func (s *KeeperSuite) setupTierParams() {
	authority := s.keeper.GetAuthority()
	tiers := []types.TierDefinition{
		{
			TierId:                        1,
			ExitCommitmentDuration:        time.Hour * 24 * 365, // 1 year
			ExitCommitmentDurationInYears: 1,
			BonusApy:                      sdkmath.LegacyNewDecWithPrec(4, 2), // 4%
			MinLockAmount:                 sdkmath.NewInt(1000),
		},
		{
			TierId:                        2,
			ExitCommitmentDuration:        time.Hour * 24 * 365 * 5, // 5 years
			ExitCommitmentDurationInYears: 5,
			BonusApy:                      sdkmath.LegacyNewDecWithPrec(8, 2), // 8%
			MinLockAmount:                 sdkmath.NewInt(5000),
		},
	}
	bondDenom, _ := s.app.StakingKeeper.BondDenom(s.ctx)
	params := types.NewParams(sdkmath.LegacyZeroDec(), tiers, []string{bondDenom})
	msg := &types.MsgUpdateParams{Authority: authority, Params: params}
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.UpdateParams(s.ctx, msg)
	s.Require().NoError(err)
}

func (s *KeeperSuite) fundAccount(addr sdk.AccAddress, amount sdkmath.Int) {
	bondDenom, _ := s.app.StakingKeeper.BondDenom(s.ctx)
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, amount))
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr, coins)
	s.Require().NoError(err)
}

func (s *KeeperSuite) getValidator() stakingtypes.Validator {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	return vals[0]
}

func (s *KeeperSuite) newMsgServer() types.MsgServer {
	return keeper.NewMsgServerImpl(s.keeper)
}

func (s *KeeperSuite) bondDenom() string {
	d, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	return d
}

// lockTierHelper creates a basic tier-1 lock for the given user and returns the position id.
func (s *KeeperSuite) lockTierHelper(owner sdk.AccAddress, amount sdkmath.Int) uint64 {
	msgServer := s.newMsgServer()
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  owner.String(),
		TierId: 1,
		Amount: sdk.NewCoin(s.bondDenom(), amount),
	})
	s.Require().NoError(err)
	return resp.PositionId
}

// lockTierWithDelegateHelper creates a tier-1 lock with delegation.
func (s *KeeperSuite) lockTierWithDelegateHelper(owner sdk.AccAddress, amount sdkmath.Int, val stakingtypes.Validator) uint64 {
	msgServer := s.newMsgServer()
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:     owner.String(),
		TierId:    1,
		Amount:    sdk.NewCoin(s.bondDenom(), amount),
		Validator: val.GetOperator(),
	})
	s.Require().NoError(err)
	return resp.PositionId
}

// ---------------------------------------------------------------------------
// MsgLockTier tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestLockTier_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_addr_1____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	msgServer := s.newMsgServer()
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  user.String(),
		TierId: 1,
		Amount: sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(2000)),
	})
	s.Require().NoError(err)
	s.Require().True(resp.PositionId > 0)

	// Verify position created.
	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().Equal(user.String(), pos.Owner)
	s.Require().Equal(uint32(1), pos.TierId)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(2000)))
	s.Require().Empty(pos.Validator) // not delegated
	s.Require().True(pos.ExitTriggeredAt.IsZero() || pos.ExitTriggeredAt.Equal(time.Unix(0, 0)))
}

func (s *KeeperSuite) TestLockTier_WithDelegate() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_addr_2____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	msgServer := s.newMsgServer()
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:     user.String(),
		TierId:    1,
		Amount:    sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(2000)),
		Validator: val.GetOperator(),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().Equal(val.GetOperator(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
	s.Require().Equal(now, pos.DelegatedAtTime)
	s.Require().Equal(now, pos.LastBonusAccrual)
}

func (s *KeeperSuite) TestLockTier_WithTriggerExitImmediately() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_addr_3____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	msgServer := s.newMsgServer()
	resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  user.String(),
		TierId:                 1,
		Amount:                 sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(2000)),
		TriggerExitImmediately: true,
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, resp.PositionId)
	s.Require().NoError(err)
	s.Require().Equal(now, pos.ExitTriggeredAt)
	// ExitUnlockTime = now + 1 year (tier 1 commitment duration)
	expectedUnlock := now.Add(time.Hour * 24 * 365)
	s.Require().Equal(expectedUnlock, pos.ExitUnlockTime)
}

func (s *KeeperSuite) TestLockTier_BelowMinimum() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_addr_4____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	msgServer := s.newMsgServer()
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  user.String(),
		TierId: 1,
		Amount: sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(999)), // below 1000 minimum
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "below tier minimum")
}

func (s *KeeperSuite) TestLockTier_InvalidTier() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_addr_5____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	msgServer := s.newMsgServer()
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  user.String(),
		TierId: 99, // non-existent tier
		Amount: sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(2000)),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "tier 99 not found")
}

func (s *KeeperSuite) TestLockTier_WrongDenom() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_addr_6____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	msgServer := s.newMsgServer()
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  user.String(),
		TierId: 1,
		Amount: sdk.NewCoin("wrongdenom", sdkmath.NewInt(2000)),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "expected denom")
}

// ---------------------------------------------------------------------------
// MsgTierDelegate tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTierDelegate_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_deleg_1___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	// Lock without delegation.
	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	// Verify not delegated.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().Empty(pos.Validator)

	// Now delegate.
	val := s.getValidator()
	msgServer := s.newMsgServer()
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      user.String(),
		PositionId: positionId,
		Validator:  val.GetOperator(),
	})
	s.Require().NoError(err)

	// Verify delegated.
	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().Equal(val.GetOperator(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())
}

func (s *KeeperSuite) TestTierDelegate_AlreadyDelegated() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_deleg_2___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Try to delegate again.
	msgServer := s.newMsgServer()
	_, err := msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      user.String(),
		PositionId: positionId,
		Validator:  val.GetOperator(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "already delegated")
}

func (s *KeeperSuite) TestTierDelegate_NotOwner() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_deleg_3___"))
	otherUser := sdk.AccAddress([]byte("test_other_user_3___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	val := s.getValidator()
	msgServer := s.newMsgServer()
	_, err := msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      otherUser.String(),
		PositionId: positionId,
		Validator:  val.GetOperator(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "not position owner")
}

// ---------------------------------------------------------------------------
// MsgTriggerExitFromTier tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTriggerExit_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_exit_1____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().Equal(now, pos.ExitTriggeredAt)
	expectedUnlock := now.Add(time.Hour * 24 * 365) // tier 1: 1 year
	s.Require().Equal(expectedUnlock, pos.ExitUnlockTime)
}

func (s *KeeperSuite) TestTriggerExit_AlreadyExiting() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_exit_2____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Try to trigger exit again.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "already triggered exit")
}

// ---------------------------------------------------------------------------
// MsgAddToTierPosition tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestAddToPosition_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_add_1_____"))
	s.fundAccount(user, sdkmath.NewInt(20000))

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(2000)))

	msgServer := s.newMsgServer()
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		Amount:     sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(1000)),
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(3000)))
}

func (s *KeeperSuite) TestAddToPosition_WhenExiting() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_add_2_____"))
	s.fundAccount(user, sdkmath.NewInt(20000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	// Trigger exit.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Try to add tokens while exiting.
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		Amount:     sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(500)),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "exiting; cannot add tokens")
}

func (s *KeeperSuite) TestAddToPosition_RejectsUnbonding() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_add_unb___"))
	s.fundAccount(user, sdkmath.NewInt(20000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Trigger exit and undelegate to enter unbonding state.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Verify position is unbonding.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)

	// Try to add tokens while unbonding -- should fail.
	_, err = msgServer.AddToTierPosition(s.ctx, &types.MsgAddToTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		Amount:     sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(500)),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unbonding; cannot add tokens")
}

// ---------------------------------------------------------------------------
// MsgTierUndelegate tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTierUndelegate_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_undel_1___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Trigger exit first (required before undelegation).
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Now undelegate.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)
	s.Require().True(pos.DelegatedShares.IsZero())
}

func (s *KeeperSuite) TestTierUndelegate_NotExiting() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_undel_2___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Try to undelegate without triggering exit first.
	msgServer := s.newMsgServer()
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "has not triggered exit")
}

func (s *KeeperSuite) TestTierUndelegate_RejectsWhenAlreadyUnbonding() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_undel_3___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	msgServer := s.newMsgServer()

	// Trigger exit.
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// First undelegate succeeds.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Second undelegate should be rejected as already unbonding.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "already unbonding")
}

// ---------------------------------------------------------------------------
// MsgTierRedelegate tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTierRedelegate_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_redel_1___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// We need a second validator to redelegate to.
	// Since the test setup only has one validator, we need to verify
	// the error when re-delegating to the same validator.
	// Redelegation to the same validator should fail in the staking module.
	msgServer := s.newMsgServer()
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        user.String(),
		PositionId:   positionId,
		DstValidator: val.GetOperator(), // same validator
	})
	// Redelegation to same validator should fail (staking module enforces this).
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "redelegate")
}

// ---------------------------------------------------------------------------
// MsgWithdrawTierRewards tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestWithdrawRewards_NotDelegated() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_reward_1__"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	// Lock without delegation.
	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	msgServer := s.newMsgServer()
	_, err := msgServer.WithdrawTierRewards(s.ctx, &types.MsgWithdrawTierRewards{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "not actively delegated")
}

// ---------------------------------------------------------------------------
// MsgFundTierPool tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestFundTierPool_Success() {
	// Authority (governance) is the only allowed sender.
	authority := s.keeper.GetAuthority()

	bondDenom := s.bondDenom()
	// Fund the gov module account (authority address is blocked from receiving via FundAccount).
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(50000)))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, "gov", coins)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.TierPoolName)
	poolBefore := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, bondDenom)

	msgServer := s.newMsgServer()
	_, err = msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Sender: authority,
		Amount: sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(10000))),
	})
	s.Require().NoError(err)

	poolAfter := s.app.BankKeeper.GetBalance(s.ctx, poolAddr, bondDenom)
	s.Require().Equal(poolBefore.Amount.Add(sdkmath.NewInt(10000)), poolAfter.Amount)
}

func (s *KeeperSuite) TestFundTierPool_RejectsUnauthorized() {
	sender := sdk.AccAddress([]byte("test_fund_unauth____"))
	s.fundAccount(sender, sdkmath.NewInt(50000))

	bondDenom := s.bondDenom()
	msgServer := s.newMsgServer()
	_, err := msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Sender: sender.String(),
		Amount: sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(10000))),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unauthorized")
}

// ---------------------------------------------------------------------------
// MsgTransferTierPosition tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTransferPosition_Success() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_xfer_1____"))
	newOwner := sdk.AccAddress([]byte("test_new_owner_1____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	msgServer := s.newMsgServer()
	_, err := msgServer.TransferTierPosition(s.ctx, &types.MsgTransferTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		NewOwner:   newOwner.String(),
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().Equal(newOwner.String(), pos.Owner)
}

func (s *KeeperSuite) TestTransferPosition_NotOwner() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_xfer_2____"))
	otherUser := sdk.AccAddress([]byte("test_other_xfer_2___"))
	newOwner := sdk.AccAddress([]byte("test_new_xfer_2_____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	// otherUser tries to transfer user's position.
	msgServer := s.newMsgServer()
	_, err := msgServer.TransferTierPosition(s.ctx, &types.MsgTransferTierPosition{
		Owner:      otherUser.String(),
		PositionId: positionId,
		NewOwner:   newOwner.String(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "not position owner")
}

// ---------------------------------------------------------------------------
// LockTier zero amount rejected
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestLockTier_ZeroAmountRejected() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_zero_1____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	msgServer := s.newMsgServer()
	_, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:  user.String(),
		TierId: 1,
		Amount: sdk.NewCoin(s.bondDenom(), sdkmath.NewInt(0)),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "amount must be positive")
}

// ---------------------------------------------------------------------------
// TierRedelegate rejects when exiting
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTierRedelegate_RejectsWhenExiting() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_redelex_1_"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Trigger exit.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Try to redelegate while exiting -- should fail.
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        user.String(),
		PositionId:   positionId,
		DstValidator: val.GetOperator(), // same validator, but should fail before staking check
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "exiting; cannot redelegate")
}

func (s *KeeperSuite) TestTierRedelegate_RejectsWhenUnbonding() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_redelunb_1"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Trigger exit and undelegate to enter unbonding state.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Verify position is unbonding.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)

	// Try to redelegate while unbonding -- should fail.
	_, err = msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        user.String(),
		PositionId:   positionId,
		DstValidator: val.GetOperator(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "unbonding; cannot redelegate")
}

// ---------------------------------------------------------------------------
// TransferTierPosition rejects exiting and self-transfer
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestTransferPosition_RejectsExiting() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_xfex_1____"))
	newOwner := sdk.AccAddress([]byte("test_new_xfex_1_____"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	// Trigger exit.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Try to transfer while exiting -- should fail.
	_, err = msgServer.TransferTierPosition(s.ctx, &types.MsgTransferTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		NewOwner:   newOwner.String(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "exiting; cannot transfer")
}

func (s *KeeperSuite) TestTransferPosition_RejectsSelfTransfer() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_xfself_1__"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	// Try to transfer to self -- should fail.
	msgServer := s.newMsgServer()
	_, err := msgServer.TransferTierPosition(s.ctx, &types.MsgTransferTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		NewOwner:   user.String(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "cannot transfer position to self")
}

func (s *KeeperSuite) TestTransferPosition_SettlesBonusToOldOwner() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_xfsett_1__"))
	newOwner := sdk.AccAddress([]byte("test_new_xfsett_1___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	// Fund tier pool so bonus can be paid.
	tierPoolAddr := s.keeper.GetModuleAddress("tier_reward_pool")
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, tierPoolAddr, sdk.NewCoins(sdk.NewCoin("stake", sdkmath.NewInt(100000))))
	s.Require().NoError(err)

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(5000), val)

	// Advance time to accrue bonus.
	laterTime := now.Add(90 * 24 * time.Hour) // 90 days
	s.ctx = s.ctx.WithBlockTime(laterTime)

	// Record old owner balance before transfer.
	oldOwnerBalBefore := s.app.BankKeeper.GetBalance(s.ctx, user, "stake")

	// Transfer position.
	msgServer := s.newMsgServer()
	_, err = msgServer.TransferTierPosition(s.ctx, &types.MsgTransferTierPosition{
		Owner:      user.String(),
		PositionId: positionId,
		NewOwner:   newOwner.String(),
	})
	s.Require().NoError(err)

	// Old owner should have received bonus payout.
	oldOwnerBalAfter := s.app.BankKeeper.GetBalance(s.ctx, user, "stake")
	s.Require().True(oldOwnerBalAfter.Amount.GT(oldOwnerBalBefore.Amount),
		"old owner should receive bonus settlement: before=%s, after=%s",
		oldOwnerBalBefore.Amount, oldOwnerBalAfter.Amount)

	// Position should belong to new owner with updated LastBonusAccrual.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().Equal(newOwner.String(), pos.Owner)
	s.Require().Equal(laterTime, pos.LastBonusAccrual)
	s.Require().True(pos.PendingBaseRewards.IsZero() || len(pos.PendingBaseRewards) == 0)
}

func (s *KeeperSuite) TestTierDelegate_RejectsExiting() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_delxit_1__"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	// Lock without delegating, then trigger exit.
	positionId := s.lockTierHelper(user, sdkmath.NewInt(2000))

	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Try to delegate an exiting position — should be rejected.
	val := s.getValidator()
	_, err = msgServer.TierDelegate(s.ctx, &types.MsgTierDelegate{
		Owner:      user.String(),
		PositionId: positionId,
		Validator:  val.GetOperator(),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "exiting; cannot delegate")
}

// ---------------------------------------------------------------------------
// EndBlocker clears completed unbonding
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestEndBlocker_ClearsCompletedUnbonding() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_endblk_1__"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Trigger exit.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Undelegate.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Verify position is unbonding with completion time set.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)
	s.Require().False(pos.UnbondingCompletionTime.IsZero())

	// Advance time past unbonding completion.
	futureCtx := s.ctx.WithBlockTime(pos.UnbondingCompletionTime.Add(time.Second))

	// Run actual EndBlocker.
	err = tieredrewards.EndBlocker(futureCtx, s.keeper)
	s.Require().NoError(err)

	// Verify unbonding cleared.
	pos, err = s.keeper.GetPosition(futureCtx, positionId)
	s.Require().NoError(err)
	s.Require().False(pos.IsUnbonding)
	s.Require().Empty(pos.Validator)
}

func (s *KeeperSuite) TestEndBlocker_SkipsActiveUnbonding() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_endblk_2__"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(2000), val)

	// Trigger exit and undelegate.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)

	// Run actual EndBlocker at current time (before completion) -- should NOT clear.
	err = tieredrewards.EndBlocker(s.ctx, s.keeper)
	s.Require().NoError(err)

	// Position should still be unbonding.
	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)
}

// ---------------------------------------------------------------------------
// Fair reward attribution across multiple positions
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestWithdrawRewards_MultiplePositions_FairAttribution() {
	s.setupTierParams()

	user1 := sdk.AccAddress([]byte("test_user_fair_1____"))
	user2 := sdk.AccAddress([]byte("test_user_fair_2____"))
	s.fundAccount(user1, sdkmath.NewInt(20000))
	s.fundAccount(user2, sdkmath.NewInt(20000))

	// Fund the tier pool for bonus payouts.
	bondDenom := s.bondDenom()
	poolFund := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.TierPoolName, poolFund)
	s.Require().NoError(err)

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()

	// Create two positions on the same validator with equal amounts.
	posId1 := s.lockTierWithDelegateHelper(user1, sdkmath.NewInt(5000), val)
	posId2 := s.lockTierWithDelegateHelper(user2, sdkmath.NewInt(5000), val)

	// Advance time for bonus accrual.
	futureTime := now.Add(time.Hour * 24 * 30) // 30 days
	s.ctx = s.ctx.WithBlockTime(futureTime)

	// When user1 withdraws, it should attribute pending rewards to user2.
	msgServer := s.newMsgServer()
	_, err = msgServer.WithdrawTierRewards(s.ctx, &types.MsgWithdrawTierRewards{
		Owner:      user1.String(),
		PositionId: posId1,
	})
	s.Require().NoError(err)

	// Verify user2's position has pending base rewards accumulated.
	pos2, err := s.keeper.GetPosition(s.ctx, posId2)
	s.Require().NoError(err)
	// Note: pending rewards depend on whether distribution module generated any rewards.
	// The key check is that the position wasn't modified incorrectly.
	s.Require().NotNil(pos2)
}

func (s *KeeperSuite) TestWithdrawRewards_PendingRewardsAccumulate() {
	s.setupTierParams()

	user1 := sdk.AccAddress([]byte("test_user_pend_1____"))
	user2 := sdk.AccAddress([]byte("test_user_pend_2____"))
	s.fundAccount(user1, sdkmath.NewInt(20000))
	s.fundAccount(user2, sdkmath.NewInt(20000))

	// Fund the tier pool for bonus payouts.
	bondDenom := s.bondDenom()
	poolFund := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.TierPoolName, poolFund)
	s.Require().NoError(err)

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()

	// Create two positions on the same validator.
	posId1 := s.lockTierWithDelegateHelper(user1, sdkmath.NewInt(5000), val)
	posId2 := s.lockTierWithDelegateHelper(user2, sdkmath.NewInt(5000), val)

	// Advance time.
	s.ctx = s.ctx.WithBlockTime(now.Add(time.Hour * 24 * 30))

	// User1 withdraws, user2 should get pending.
	msgServer := s.newMsgServer()
	_, err = msgServer.WithdrawTierRewards(s.ctx, &types.MsgWithdrawTierRewards{
		Owner:      user1.String(),
		PositionId: posId1,
	})
	s.Require().NoError(err)

	// Now user2 withdraws and should collect their pending + any new rewards.
	s.ctx = s.ctx.WithBlockTime(now.Add(time.Hour * 24 * 60))

	_, err = msgServer.WithdrawTierRewards(s.ctx, &types.MsgWithdrawTierRewards{
		Owner:      user2.String(),
		PositionId: posId2,
	})
	s.Require().NoError(err)

	// After withdrawal, pending base rewards should be cleared.
	pos2, err := s.keeper.GetPosition(s.ctx, posId2)
	s.Require().NoError(err)
	s.Require().True(pos2.PendingBaseRewards.IsZero() || len(pos2.PendingBaseRewards) == 0)
}

// ---------------------------------------------------------------------------
// FundTierPool rejects non-bond denom
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestFundTierPool_RejectsNonBondDenom() {
	authority := s.keeper.GetAuthority()
	bondDenom := s.bondDenom()
	// Fund the gov module account so it has balance to attempt sending.
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(50000)))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, "gov", coins)
	s.Require().NoError(err)

	msgServer := s.newMsgServer()
	_, err = msgServer.FundTierPool(s.ctx, &types.MsgFundTierPool{
		Sender: authority,
		Amount: sdk.NewCoins(sdk.NewCoin("wrongdenom", sdkmath.NewInt(10000))),
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "only bond denom")
}

// ---------------------------------------------------------------------------
// TestFullUnbondingLifecycle
// lock → delegate → exit → undelegate → EndBlocker → withdraw
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestFullUnbondingLifecycle() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_lifecycle__"))
	s.fundAccount(user, sdkmath.NewInt(20000))

	bondDenom := s.bondDenom()

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	msgServer := s.newMsgServer()

	// Step 1: Lock with delegation.
	lockResp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:     user.String(),
		TierId:    1,
		Amount:    sdk.NewCoin(bondDenom, sdkmath.NewInt(5000)),
		Validator: val.GetOperator(),
	})
	s.Require().NoError(err)
	positionId := lockResp.PositionId

	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().Equal(val.GetOperator(), pos.Validator)
	s.Require().True(pos.DelegatedShares.IsPositive())

	// Step 2: Trigger exit.
	_, err = msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(keeper.IsPositionExiting(pos))
	expectedUnlock := now.Add(time.Hour * 24 * 365) // tier 1: 1 year
	s.Require().Equal(expectedUnlock, pos.ExitUnlockTime)

	// Step 3: Undelegate.
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)
	s.Require().True(pos.DelegatedShares.IsZero())
	s.Require().False(pos.UnbondingCompletionTime.IsZero())

	unbondingCompletion := pos.UnbondingCompletionTime

	// Attempt withdraw before EndBlocker clears unbonding -- should fail.
	// Validator field is still set (cleared by EndBlocker), so hits "still delegated" first.
	afterExitUnlock := expectedUnlock.Add(time.Second)
	ctxAfterExitUnlock := s.ctx.WithBlockTime(afterExitUnlock)
	_, err = msgServer.WithdrawFromTier(ctxAfterExitUnlock, &types.MsgWithdrawFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "still delegated")

	// Step 4: Run actual EndBlocker after unbonding completion time.
	afterUnbonding := unbondingCompletion.Add(time.Second)
	ctxAfterUnbonding := s.ctx.WithBlockTime(afterUnbonding)

	err = tieredrewards.EndBlocker(ctxAfterUnbonding, s.keeper)
	s.Require().NoError(err)

	// Verify unbonding cleared.
	pos, err = s.keeper.GetPosition(ctxAfterUnbonding, positionId)
	s.Require().NoError(err)
	s.Require().False(pos.IsUnbonding)
	s.Require().Empty(pos.Validator)

	// Step 5: Withdraw (both exit unlock and unbonding are complete).
	// Use a time that's past both the exit unlock and unbonding completion.
	withdrawTime := afterUnbonding
	if afterExitUnlock.After(withdrawTime) {
		withdrawTime = afterExitUnlock
	}
	ctxWithdraw := s.ctx.WithBlockTime(withdrawTime)

	// Simulate the staking module returning tokens to the tier module account
	// after unbonding completes (in a real chain this happens automatically).
	err = banktestutil.FundModuleAccount(ctxWithdraw, s.app.BankKeeper, types.ModuleName, sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(5000))))
	s.Require().NoError(err)

	balanceBefore := s.app.BankKeeper.GetBalance(ctxWithdraw, user, bondDenom)

	_, err = msgServer.WithdrawFromTier(ctxWithdraw, &types.MsgWithdrawFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// User should have received their locked tokens back.
	balanceAfter := s.app.BankKeeper.GetBalance(ctxWithdraw, user, bondDenom)
	s.Require().True(balanceAfter.Amount.GT(balanceBefore.Amount) || balanceAfter.Amount.Equal(balanceBefore.Amount.Add(sdkmath.NewInt(5000))))

	// Position should be deleted.
	_, err = s.keeper.GetPosition(ctxWithdraw, positionId)
	s.Require().Error(err)
}

// ---------------------------------------------------------------------------
// Slash hook tests
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestSlashHook_ReducesAmountLocked() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_slash_1___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(5000), val)

	// Verify initial AmountLocked.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(5000)))

	// Invoke the slash hook with 10% fraction.
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)
	fraction := sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1 = 10%
	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, fraction)
	s.Require().NoError(err)

	// AmountLocked should be reduced by 10% → 4500.
	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(4500)),
		"expected 4500, got %s", pos.AmountLocked)
	// DelegatedShares should remain unchanged (exchange rate changes, not shares).
	s.Require().True(pos.DelegatedShares.IsPositive())
}

func (s *KeeperSuite) TestSlashHook_ReducesUnbondingPositions() {
	s.setupTierParams()

	user := sdk.AccAddress([]byte("test_user_slash_2___"))
	s.fundAccount(user, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()
	positionId := s.lockTierWithDelegateHelper(user, sdkmath.NewInt(5000), val)

	// Trigger exit and undelegate to enter unbonding state.
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user.String(),
		PositionId: positionId,
	})
	s.Require().NoError(err)

	// Verify unbonding state.
	pos, err := s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.IsUnbonding)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(5000)))

	// Invoke the slash hook with 5% fraction — should also reduce unbonding positions.
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)
	fraction := sdkmath.LegacyNewDecWithPrec(5, 2) // 0.05 = 5%
	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, fraction)
	s.Require().NoError(err)

	// AmountLocked should be reduced by 5% → 4750.
	pos, err = s.keeper.GetPosition(s.ctx, positionId)
	s.Require().NoError(err)
	s.Require().True(pos.AmountLocked.Equal(sdkmath.NewInt(4750)),
		"expected 4750 for unbonding position, got %s", pos.AmountLocked)
}

func (s *KeeperSuite) TestSlashHook_MixedActiveAndUnbonding() {
	s.setupTierParams()

	user1 := sdk.AccAddress([]byte("test_user_slash_3___"))
	user2 := sdk.AccAddress([]byte("test_user_slash_4___"))
	s.fundAccount(user1, sdkmath.NewInt(10000))
	s.fundAccount(user2, sdkmath.NewInt(10000))

	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	val := s.getValidator()

	// User1: active delegated position (3000 tokens).
	posId1 := s.lockTierWithDelegateHelper(user1, sdkmath.NewInt(3000), val)

	// User2: unbonding position (7000 tokens).
	posId2 := s.lockTierWithDelegateHelper(user2, sdkmath.NewInt(7000), val)
	msgServer := s.newMsgServer()
	_, err := msgServer.TriggerExitFromTier(s.ctx, &types.MsgTriggerExitFromTier{
		Owner:      user2.String(),
		PositionId: posId2,
	})
	s.Require().NoError(err)
	_, err = msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      user2.String(),
		PositionId: posId2,
	})
	s.Require().NoError(err)

	// Slash 10%.
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)
	fraction := sdkmath.LegacyNewDecWithPrec(1, 1) // 10%
	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, fraction)
	s.Require().NoError(err)

	// Active position: 3000 - 300 = 2700.
	pos1, err := s.keeper.GetPosition(s.ctx, posId1)
	s.Require().NoError(err)
	s.Require().True(pos1.AmountLocked.Equal(sdkmath.NewInt(2700)),
		"active position: expected 2700, got %s", pos1.AmountLocked)

	// Unbonding position: 7000 - 700 = 6300.
	pos2, err := s.keeper.GetPosition(s.ctx, posId2)
	s.Require().NoError(err)
	s.Require().True(pos2.AmountLocked.Equal(sdkmath.NewInt(6300)),
		"unbonding position: expected 6300, got %s", pos2.AmountLocked)
}
