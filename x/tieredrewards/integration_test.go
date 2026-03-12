package tieredrewards_test

import (
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// setupTierParams configures a single tier and returns the msg server and bond denom.
func setupTierParams(t *testing.T) (
	ctx sdk.Context,
	msgServer types.MsgServer,
	bondDenom string,
	a *app.ChainApp,
) {
	t.Helper()
	app := testutil.Setup(false, nil)
	ctx = app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})
	// Set a deterministic block time so time-based assertions are stable.
	ctx = ctx.WithBlockTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	bondDenom, err := app.StakingKeeper.BondDenom(ctx)
	require.NoError(t, err)

	authority := app.TieredRewardsKeeper.GetAuthority()
	tiers := []types.TierDefinition{{
		TierId:                        1,
		ExitCommitmentDuration:        time.Hour * 24 * 365, // 1 year
		ExitCommitmentDurationInYears: 1,
		BonusApy:                      sdkmath.LegacyNewDecWithPrec(4, 2), // 4%
		MinLockAmount:                 sdkmath.NewInt(1000),
	}}
	params := types.NewParams(sdkmath.LegacyZeroDec(), tiers, []string{bondDenom})

	msgServer = keeper.NewMsgServerImpl(app.TieredRewardsKeeper)
	_, err = msgServer.UpdateParams(ctx, &types.MsgUpdateParams{Authority: authority, Params: params})
	require.NoError(t, err)

	return ctx, msgServer, bondDenom, app
}

// TestFullLockDelegateExitFlow exercises the complete lifecycle:
// lock -> delegate -> trigger exit -> fail early withdraw -> undelegate -> verify unbonding.
func TestFullLockDelegateExitFlow(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	// Create and fund a user.
	userAddr := sdk.AccAddress([]byte("tier_test_user_addr1"))
	lockAmount := sdkmath.NewInt(10000)
	err := banktestutil.FundAccount(ctx, a.BankKeeper, userAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	require.NoError(t, err)

	// Fund the tier pool for bonus payouts.
	poolFund := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)))
	err = banktestutil.FundModuleAccount(ctx, a.BankKeeper, types.RewardsPoolName, poolFund)
	require.NoError(t, err)

	// Step 1: Lock tokens into tier 1.
	lockResp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  userAddr.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, lockAmount),
	})
	require.NoError(t, err)
	positionId := lockResp.PositionId

	// Verify position created with expected state.
	pos, err := a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.Equal(t, lockAmount, pos.AmountLocked)
	require.Empty(t, pos.Validator) // not delegated yet

	// Step 2: Delegate to a validator.
	vals, err := a.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	valAddr := vals[0].GetOperator()

	_, err = msgServer.TierDelegate(ctx, &types.MsgTierDelegate{
		Owner:      userAddr.String(),
		PositionId: positionId,
		Validator:  valAddr,
	})
	require.NoError(t, err)

	// Verify delegation recorded.
	pos, err = a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.NotEmpty(t, pos.Validator)
	require.True(t, pos.DelegatedShares.IsPositive())

	// Step 3: Trigger exit.
	_, err = msgServer.TriggerExitFromTier(ctx, &types.MsgTriggerExitFromTier{
		Owner:      userAddr.String(),
		PositionId: positionId,
	})
	require.NoError(t, err)

	pos, err = a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.False(t, pos.ExitTriggeredAt.IsZero())

	// Step 4: Attempt to withdraw before exit commitment has elapsed -- should fail.
	_, err = msgServer.WithdrawFromTier(ctx, &types.MsgWithdrawFromTier{
		Owner:      userAddr.String(),
		PositionId: positionId,
	})
	require.Error(t, err) // exit commitment not elapsed

	// Step 5: Undelegate (allowed because exit is triggered).
	_, err = msgServer.TierUndelegate(ctx, &types.MsgTierUndelegate{
		Owner:      userAddr.String(),
		PositionId: positionId,
	})
	require.NoError(t, err)

	// Verify position is now unbonding.
	pos, err = a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.True(t, pos.IsUnbonding)
}

// TestLockWithDelegateAndExitAtCreation tests lock with both validator and
// trigger_exit_immediately options set at creation time.
func TestLockWithDelegateAndExitAtCreation(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	userAddr := sdk.AccAddress([]byte("tier_test_user_addr2"))
	lockAmount := sdkmath.NewInt(5000)
	err := banktestutil.FundAccount(ctx, a.BankKeeper, userAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	require.NoError(t, err)

	vals, err := a.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	valAddr := vals[0].GetOperator()

	// Lock with delegation and immediate exit trigger in one call.
	lockResp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:                  userAddr.String(),
		TierId:                 1,
		Amount:                 sdk.NewCoin(bondDenom, lockAmount),
		Validator:              valAddr,
		TriggerExitImmediately: true,
	})
	require.NoError(t, err)

	pos, err := a.TieredRewardsKeeper.GetPosition(ctx, lockResp.PositionId)
	require.NoError(t, err)

	// Verify delegation was set.
	require.Equal(t, valAddr, pos.Validator)
	require.True(t, pos.DelegatedShares.IsPositive())

	// Verify exit was triggered.
	require.False(t, pos.ExitTriggeredAt.IsZero())
	require.False(t, pos.ExitUnlockTime.IsZero())

	// ExitUnlockTime should be ~1 year after block time.
	expectedUnlock := ctx.BlockTime().Add(time.Hour * 24 * 365)
	require.Equal(t, expectedUnlock, pos.ExitUnlockTime)
}

// TestAddToPositionFlow tests lock -> add tokens -> verify increased amount.
func TestAddToPositionFlow(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	userAddr := sdk.AccAddress([]byte("tier_test_user_addr3"))
	initialAmount := sdkmath.NewInt(5000)
	addAmount := sdkmath.NewInt(3000)
	totalFund := initialAmount.Add(addAmount)
	err := banktestutil.FundAccount(ctx, a.BankKeeper, userAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, totalFund)))
	require.NoError(t, err)

	// Lock initial amount.
	lockResp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  userAddr.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, initialAmount),
	})
	require.NoError(t, err)
	positionId := lockResp.PositionId

	pos, err := a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.Equal(t, initialAmount, pos.AmountLocked)

	// Add more tokens to the position.
	_, err = msgServer.AddToTierPosition(ctx, &types.MsgAddToTierPosition{
		Owner:      userAddr.String(),
		PositionId: positionId,
		Amount:     sdk.NewCoin(bondDenom, addAmount),
	})
	require.NoError(t, err)

	// Verify increased amount.
	pos, err = a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.Equal(t, totalFund, pos.AmountLocked)
}

// TestAddToPositionRejectsWhenExiting tests that adding tokens to an exiting
// position is rejected.
func TestAddToPositionRejectsWhenExiting(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	userAddr := sdk.AccAddress([]byte("tier_test_user_addr4"))
	err := banktestutil.FundAccount(ctx, a.BankKeeper, userAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(10000))))
	require.NoError(t, err)

	// Lock.
	lockResp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  userAddr.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, sdkmath.NewInt(5000)),
	})
	require.NoError(t, err)

	// Trigger exit.
	_, err = msgServer.TriggerExitFromTier(ctx, &types.MsgTriggerExitFromTier{
		Owner:      userAddr.String(),
		PositionId: lockResp.PositionId,
	})
	require.NoError(t, err)

	// Attempt to add tokens -- should fail because position is exiting.
	_, err = msgServer.AddToTierPosition(ctx, &types.MsgAddToTierPosition{
		Owner:      userAddr.String(),
		PositionId: lockResp.PositionId,
		Amount:     sdk.NewCoin(bondDenom, sdkmath.NewInt(1000)),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exiting")
}

// TestFundTierPoolAndQueryBalance tests funding the tier bonus pool and
// verifying the balance via the query.
func TestFundTierPoolAndQueryBalance(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	// FundTierPool is restricted to the governance authority.
	authority := a.TieredRewardsKeeper.GetAuthority()
	fundAmount := sdkmath.NewInt(50000)
	err := banktestutil.FundModuleAccount(ctx, a.BankKeeper, "gov", sdk.NewCoins(sdk.NewCoin(bondDenom, fundAmount)))
	require.NoError(t, err)

	// Fund the tier pool via message.
	_, err = msgServer.FundTierPool(ctx, &types.MsgFundTierPool{
		Sender: authority,
		Amount: sdk.NewCoins(sdk.NewCoin(bondDenom, fundAmount)),
	})
	require.NoError(t, err)

	// Verify pool balance by querying the TierPoolName module account directly.
	poolAddr := a.TieredRewardsKeeper.GetModuleAddress(types.TierPoolName)
	balance := a.BankKeeper.GetBalance(ctx, poolAddr, bondDenom)
	require.Equal(t, fundAmount, balance.Amount)
}

// TestTransferPositionFlow tests transferring a position to a new owner.
func TestTransferPositionFlow(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	originalOwner := sdk.AccAddress([]byte("tier_test_user_addr6"))
	newOwner := sdk.AccAddress([]byte("tier_test_user_addr7"))
	lockAmount := sdkmath.NewInt(5000)
	err := banktestutil.FundAccount(ctx, a.BankKeeper, originalOwner, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	require.NoError(t, err)

	// Lock a position.
	lockResp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  originalOwner.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, lockAmount),
	})
	require.NoError(t, err)
	positionId := lockResp.PositionId

	// Transfer to new owner.
	_, err = msgServer.TransferTierPosition(ctx, &types.MsgTransferTierPosition{
		Owner:      originalOwner.String(),
		PositionId: positionId,
		NewOwner:   newOwner.String(),
	})
	require.NoError(t, err)

	// Verify ownership changed.
	pos, err := a.TieredRewardsKeeper.GetPosition(ctx, positionId)
	require.NoError(t, err)
	require.Equal(t, newOwner.String(), pos.Owner)

	// Verify new owner can trigger exit.
	_, err = msgServer.TriggerExitFromTier(ctx, &types.MsgTriggerExitFromTier{
		Owner:      newOwner.String(),
		PositionId: positionId,
	})
	require.NoError(t, err)

	// Verify original owner can no longer operate on the position.
	_, err = msgServer.TriggerExitFromTier(ctx, &types.MsgTriggerExitFromTier{
		Owner:      originalOwner.String(),
		PositionId: positionId,
	})
	require.Error(t, err)
}

// TestMultiplePositionsPerOwner tests creating multiple independent positions
// for the same owner and verifying they have independent state.
func TestMultiplePositionsPerOwner(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	userAddr := sdk.AccAddress([]byte("tier_test_user_addr8"))
	err := banktestutil.FundAccount(ctx, a.BankKeeper, userAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(20000))))
	require.NoError(t, err)

	// Create two positions.
	lockResp1, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  userAddr.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, sdkmath.NewInt(8000)),
	})
	require.NoError(t, err)

	lockResp2, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  userAddr.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, sdkmath.NewInt(5000)),
	})
	require.NoError(t, err)

	require.NotEqual(t, lockResp1.PositionId, lockResp2.PositionId)

	// Trigger exit only on position 1.
	_, err = msgServer.TriggerExitFromTier(ctx, &types.MsgTriggerExitFromTier{
		Owner:      userAddr.String(),
		PositionId: lockResp1.PositionId,
	})
	require.NoError(t, err)

	// Position 1 should be exiting.
	pos1, err := a.TieredRewardsKeeper.GetPosition(ctx, lockResp1.PositionId)
	require.NoError(t, err)
	require.False(t, pos1.ExitTriggeredAt.IsZero())

	// Position 2 should NOT be exiting.
	pos2, err := a.TieredRewardsKeeper.GetPosition(ctx, lockResp2.PositionId)
	require.NoError(t, err)
	require.True(t, pos2.ExitTriggeredAt.IsZero())

	// Verify both positions appear in owner query.
	positions, err := a.TieredRewardsKeeper.GetPositionsByOwner(ctx, userAddr.String())
	require.NoError(t, err)
	require.Len(t, positions, 2)
}

// TestTierUndelegateRejectsWhenNotExiting tests that undelegation is rejected
// when exit has not been triggered yet.
func TestTierUndelegateRejectsWhenNotExiting(t *testing.T) {
	ctx, msgServer, bondDenom, a := setupTierParams(t)

	userAddr := sdk.AccAddress([]byte("tier_test_user_addr9"))
	lockAmount := sdkmath.NewInt(5000)
	err := banktestutil.FundAccount(ctx, a.BankKeeper, userAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	require.NoError(t, err)

	// Lock.
	lockResp, err := msgServer.LockTier(ctx, &types.MsgLockTier{
		Owner:  userAddr.String(),
		TierId: 1,
		Amount: sdk.NewCoin(bondDenom, lockAmount),
	})
	require.NoError(t, err)

	// Delegate.
	vals, err := a.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	valAddr := vals[0].GetOperator()

	_, err = msgServer.TierDelegate(ctx, &types.MsgTierDelegate{
		Owner:      userAddr.String(),
		PositionId: lockResp.PositionId,
		Validator:  valAddr,
	})
	require.NoError(t, err)

	// Attempt to undelegate without triggering exit first -- should fail.
	_, err = msgServer.TierUndelegate(ctx, &types.MsgTierUndelegate{
		Owner:      userAddr.String(),
		PositionId: lockResp.PositionId,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must trigger exit before undelegating")
}
