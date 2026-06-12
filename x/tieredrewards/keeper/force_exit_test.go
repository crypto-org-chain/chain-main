// Can be deleted after v8 upgrade
package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	migration "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/migrations/v2"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// totalDelegated returns the sum (in tokens) of all on-chain delegations for an
// address across every validator. Used by the migration tests to assert the
// invariant DV + DF == Σ delegations after force-exit.
func (s *KeeperSuite) totalDelegated(addr sdk.AccAddress) sdkmath.Int {
	s.T().Helper()
	delegations, err := s.app.StakingKeeper.GetDelegatorDelegations(s.ctx, addr, 1000)
	s.Require().NoError(err)
	total := sdkmath.ZeroInt()
	for _, d := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(d.GetValidatorAddr())
		s.Require().NoError(err)
		val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
		s.Require().NoError(err)
		total = total.Add(val.TokensFromShares(d.Shares).TruncateInt())
	}
	return total
}

// delegatedFree returns the DelegatedFree field of a vesting account.
func (s *KeeperSuite) delegatedFree(addr sdk.AccAddress) sdk.Coins {
	s.T().Helper()
	acc, ok := s.app.AccountKeeper.GetAccount(s.ctx, addr).(interface {
		GetDelegatedFree() sdk.Coins
	})
	s.Require().True(ok, "account must be a vesting account")
	return acc.GetDelegatedFree()
}

// trackedTotal returns DelegatedVesting + DelegatedFree (in bond denom).
func (s *KeeperSuite) trackedTotal(addr sdk.AccAddress, bondDenom string) sdkmath.Int {
	s.T().Helper()
	dv := s.delegatedVesting(addr).AmountOf(bondDenom)
	df := s.delegatedFree(addr).AmountOf(bondDenom)
	return dv.Add(df)
}

// createCommitPositionV1 simulates the CommitDelegationToTier path for a
// vesting owner: the owner first delegates normally (so DV+DF reflects the
// delegation), then transferDelegationToPosition moves the delegation into a
// tier position. This leaves DV+DF stale-high relative to actual delegations.
func (s *KeeperSuite) createCommitPositionV1(
	owner sdk.AccAddress, val stakingtypes.Validator, valAddr sdk.ValAddress, amount sdkmath.Int,
) types.PositionState {
	s.T().Helper()
	_, err := s.app.StakingKeeper.Delegate(s.ctx, owner, amount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	posDelAddr := sdk.MustAccAddressFromBech32(migration.LegacyDelegatorAddress(s.peekNextPositionId()))
	posDelAcc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, posDelAddr)
	s.app.AccountKeeper.SetAccount(s.ctx, posDelAcc)

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, owner.String(), posDelAddr, valAddr.String(), amount)
	s.Require().NoError(err)

	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	pos, err := s.keeper.CreateDelegatedPosition(s.ctx, owner.String(), tier, valAddr, posDelAddr, false)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	return state
}

// createLockTierPositionV1 simulates the LockTier path for a vesting owner:
// bank-send spendable coins from owner into the position delegator, then
// delegate from the position delegator. The owner's DV/DF are not touched.
func (s *KeeperSuite) createLockTierPositionV1(
	owner sdk.AccAddress, valAddr sdk.ValAddress, amount sdkmath.Int,
) types.PositionState {
	s.T().Helper()
	posDelAddr := sdk.MustAccAddressFromBech32(migration.LegacyDelegatorAddress(s.peekNextPositionId()))
	posDelAcc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, posDelAddr)
	s.app.AccountKeeper.SetAccount(s.ctx, posDelAcc)

	s.Require().NoError(s.keeper.LockFunds(s.ctx, owner, posDelAddr, amount))

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, posDelAddr, amount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	pos, err := s.keeper.CreateDelegatedPosition(s.ctx, owner.String(), tier, valAddr, posDelAddr, false)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))

	state, err := s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	return state
}

// newVestingOwnerWithBalance creates a fresh PermanentLockedAccount address
// funded with `balance` of bondDenom and returns the address.
func (s *KeeperSuite) newVestingOwnerWithBalance(bondDenom string, originalVesting, balance sdkmath.Int) sdk.AccAddress {
	s.T().Helper()
	owner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	baseAcc, ok := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, owner).(*authtypes.BaseAccount)
	s.Require().True(ok)
	vestingAcc, err := vestingtypes.NewPermanentLockedAccount(
		baseAcc, sdk.NewCoins(sdk.NewCoin(bondDenom, originalVesting)),
	)
	s.Require().NoError(err)
	s.app.AccountKeeper.SetAccount(s.ctx, vestingAcc)
	s.Require().NoError(banktestutil.FundAccount(
		s.ctx, s.app.BankKeeper, owner, sdk.NewCoins(sdk.NewCoin(bondDenom, balance)),
	))
	return owner
}

// advanceForRewards moves the chain forward and allocates staking + bonus
// rewards so claimRewards inside ForceFullExitWithDelegation has something to
// withdraw.
func (s *KeeperSuite) advanceForRewards(valAddr sdk.ValAddress, bondDenom string) {
	s.T().Helper()
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(100_000_000), bondDenom)
}

// TestForceFullExitWithDelegation_VestingOwner_CommitOrigin verifies the
// scenario where the vesting account's tier position originated from
// CommitDelegationToTier. The owner first delegated normally (so DV was
// populated by the bank-side TrackDelegation hook), then moved that delegation
// into a position. Pre-migration, DV+DF is stale-high. After force-exit, the
// returning delegation closes the gap; no top-up is required and DV+DF must
// remain unchanged.
func (s *KeeperSuite) TestForceFullExitWithDelegation_VestingOwner_CommitOrigin() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockedAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	owner := s.newVestingOwnerWithBalance(bondDenom, lockedAmount, lockedAmount)

	pos := s.createCommitPositionV1(owner, val, valAddr, lockedAmount)
	initialDV := s.delegatedVesting(owner).AmountOf(bondDenom)
	initialDF := s.delegatedFree(owner).AmountOf(bondDenom)
	s.Require().Equal(lockedAmount, initialDV,
		"after the original Delegate, DV must equal the delegated locked amount")
	s.Require().True(initialDF.IsZero(), "DF must be zero for a fully-locked delegation")

	// Pre-migration: delegation is on the position, owner has none. DV+DF
	// is stale-high relative to the owner's actual delegations.
	s.Require().True(s.totalDelegated(owner).IsZero(),
		"owner has no on-chain delegation pre force-exit; tier position holds it")
	s.Require().Equal(lockedAmount, s.trackedTotal(owner, bondDenom),
		"DV+DF should be stale-high (= lockedAmount) before force exit")

	s.advanceForRewards(valAddr, bondDenom)

	s.Require().NoError(s.keeper.ForceFullExitWithDelegation(s.ctx, pos.Id))

	// Position deleted, delegation back at the owner.
	_, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err)
	ownerDel := s.totalDelegated(owner)
	s.Require().Equal(lockedAmount, ownerDel,
		"owner must hold a delegation equal to the original locked amount")

	// DV unchanged: the stale-high tracking now correctly reflects the
	// returned delegation. No top-up was needed.
	s.Require().Equal(initialDV, s.delegatedVesting(owner).AmountOf(bondDenom),
		"DV must remain unchanged across the round trip")
	s.Require().Equal(initialDF, s.delegatedFree(owner).AmountOf(bondDenom),
		"DF must remain unchanged across the round trip")

	// Final invariant.
	s.Require().Equal(ownerDel, s.trackedTotal(owner, bondDenom),
		"DV + DF must equal Σ delegations after force exit")
}

// TestForceFullExitWithDelegation_VestingOwner_LockOrigin verifies the scenario
// where the vesting account's tier position originated from LockTier. The
// owner sent spendable bank coins to the position delegator; DV/DF were never
// touched. After force-exit returns the delegation to the owner, DV+DF would
// be stale-low without alignment. The migration must top up DV+DF by the
// position amount via TrackDelegation.
func (s *KeeperSuite) TestForceFullExitWithDelegation_VestingOwner_LockOrigin() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	lockedAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	// OriginalVesting equals the locked amount; the account is funded with
	// 2*lockedAmount so that lockedAmount of it is spendable (balance −
	// LockedCoins). LockTier consumes the spendable portion via bank send,
	// without touching DelegatedVesting/DelegatedFree.
	owner := s.newVestingOwnerWithBalance(bondDenom, lockedAmount, lockedAmount.MulRaw(2))

	pos := s.createLockTierPositionV1(owner, valAddr, lockedAmount)

	// Pre-migration: DV/DF are zero (LockTier didn't touch them); owner has
	// no on-chain delegation.
	s.Require().True(s.delegatedVesting(owner).AmountOf(bondDenom).IsZero(),
		"DV must be zero for a LockTier-origin position before force exit")
	s.Require().True(s.delegatedFree(owner).AmountOf(bondDenom).IsZero(),
		"DF must be zero for a LockTier-origin position before force exit")
	s.Require().True(s.totalDelegated(owner).IsZero(),
		"owner has no on-chain delegation pre force-exit; position holds it")

	s.advanceForRewards(valAddr, bondDenom)

	s.Require().NoError(s.keeper.ForceFullExitWithDelegation(s.ctx, pos.Id))

	_, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err)
	ownerDel := s.totalDelegated(owner)
	s.Require().Equal(lockedAmount, ownerDel,
		"owner must hold the returned delegation post force-exit")

	// Alignment must have topped up DV+DF by lockedAmount; otherwise a later
	// normal Undelegate would underflow vesting accounting.
	s.Require().Equal(lockedAmount, s.delegatedVesting(owner).AmountOf(bondDenom),
		"DV must saturate at OriginalVesting (= lockedAmount)")
	s.Require().True(s.delegatedFree(owner).AmountOf(bondDenom).IsZero(),
		"DF must be zero — the deficit fits entirely within OriginalVesting")
	s.Require().Equal(ownerDel, s.trackedTotal(owner, bondDenom),
		"alignment must satisfy DV + DF == Σ delegations for LockTier-origin positions")
}

// TestForceFullExitWithDelegation_VestingOwner_Mixed verifies that when an
// owner has both a CommitDelegationToTier-origin and a LockTier-origin
// position, the per-position alignment is order-independent. Whichever
// position is exited first either (a) consumes the stale-high DV+DF leaving a
// zero gap, or (b) opens a deficit gap that the second exit's alignment closes
// to match Σ delegations. Either order yields the same final state with
// DV+DF == Σ delegations.
func (s *KeeperSuite) TestForceFullExitWithDelegation_VestingOwner_Mixed() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	commitAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64() * 2)
	totalAmount := commitAmount.Add(lockAmount)

	// Balance must cover both: lockAmount for LockTier, commitAmount for the
	// initial Delegate() call that establishes the commit-origin position.
	owner := s.newVestingOwnerWithBalance(bondDenom, commitAmount, totalAmount)

	commitPos := s.createCommitPositionV1(owner, val, valAddr, commitAmount)
	lockPos := s.createLockTierPositionV1(owner, valAddr, lockAmount)

	// Pre-migration sanity: DV+DF reflects only the commit-origin amount.
	s.Require().Equal(commitAmount, s.trackedTotal(owner, bondDenom),
		"only the commit-origin position contributes to pre-migration DV+DF")
	s.Require().True(s.totalDelegated(owner).IsZero(),
		"owner's on-chain delegation count is zero pre force-exit")

	s.advanceForRewards(valAddr, bondDenom)

	// Exit commit-origin first: closes the stale-high gap (no top-up needed).
	s.Require().NoError(s.keeper.ForceFullExitWithDelegation(s.ctx, commitPos.Id))

	delAfterFirst := s.totalDelegated(owner)
	s.Require().Equal(commitAmount, delAfterFirst,
		"after first exit, only the commit-origin delegation is back")
	s.Require().Equal(commitAmount, s.delegatedVesting(owner).AmountOf(bondDenom))
	s.Require().True(s.delegatedFree(owner).AmountOf(bondDenom).IsZero())
	s.Require().Equal(delAfterFirst, s.trackedTotal(owner, bondDenom),
		"after commit-origin exit, DV+DF should already match Σ delegations")

	// Exit lock-origin next: opens a deficit that alignment must close.
	s.Require().NoError(s.keeper.ForceFullExitWithDelegation(s.ctx, lockPos.Id))

	finalDel := s.totalDelegated(owner)
	s.Require().Equal(totalAmount, finalDel,
		"after both exits, owner must hold the sum of both position amounts as delegation")
	s.Require().Equal(commitAmount, s.delegatedVesting(owner).AmountOf(bondDenom))
	s.Require().True(lockAmount.Equal(s.delegatedFree(owner).AmountOf(bondDenom)))
	s.Require().Equal(finalDel, s.trackedTotal(owner, bondDenom),
		"final DV + DF must equal Σ delegations regardless of position-origin mix")

	// Spendable rewards arrive at the owner's bank and are not artificially
	// locked — locked portion stays bounded by OriginalVesting, with DV
	// saturating against it.
	spendable := s.app.BankKeeper.SpendableCoins(s.ctx, owner)
	bal := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom).Amount
	s.Require().True(spendable.AmountOf(bondDenom).LTE(bal),
		"spendable cannot exceed bank balance")
}
