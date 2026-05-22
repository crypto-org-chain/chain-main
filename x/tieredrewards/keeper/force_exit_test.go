// Can be deleted after v7.3.0 upgrade
package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperSuite) TestForceFullExitWithDelegation_VestingOwner() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	// Use DefaultPowerReduction so distribution math doesn't truncate the
	// position's share of the validator's reward pool to zero. Set
	// validator commission to zero so 100% of allocated rewards reach
	// delegators.
	lockedAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	lockedCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, lockedAmount))
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	owner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	baseAcc, ok := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, owner).(*authtypes.BaseAccount)
	s.Require().True(ok)
	vestingAcc, err := vestingtypes.NewPermanentLockedAccount(baseAcc, lockedCoins)
	s.Require().NoError(err)
	s.app.AccountKeeper.SetAccount(s.ctx, vestingAcc)
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, owner, lockedCoins))

	_, err = s.app.StakingKeeper.Delegate(s.ctx, owner, lockedAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)
	initialDV := s.delegatedVesting(owner)
	s.Require().Equal(lockedCoins.String(), initialDV.String(),
		"DelegatedVesting must equal the delegated locked amount after a normal delegate")

	posDelAddr := types.GetDelegatorAddress(0)
	posDelAcc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, posDelAddr)
	s.app.AccountKeeper.SetAccount(s.ctx, posDelAcc)

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, owner.String(), posDelAddr, valAddr.String(), lockedAmount)
	s.Require().NoError(err)
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	pos, err := s.keeper.CreateDelegatedPosition(s.ctx, owner.String(), tier, valAddr, false)
	s.Require().NoError(err)
	s.Require().NoError(s.keeper.SetPosition(s.ctx, pos, &keeper.ValidatorTransition{PreviousAddress: ""}))
	s.Require().NoError(s.app.DistrKeeper.SetWithdrawAddr(s.ctx, posDelAddr, owner))

	// Sanity: post-bypass, owner has no staking delegation; the position holds it.
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, owner, valAddr)
	s.Require().Error(err, "owner delegation should be gone after the bypass commit")
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(posDel.Shares.IsPositive())

	// Advance block + time so distribution period progresses, then allocate
	// staking rewards to the validator and fund the bonus rewards pool. Both
	// types of rewards are claimed during ForceFullExitWithDelegation.
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1000), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(100_000_000), bondDenom)

	balBefore := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom).Amount
	s.Require().True(balBefore.IsZero(), "owner bank balance should be zero before force exit")

	// Run the migration mechanism.
	s.Require().NoError(s.keeper.ForceFullExitWithDelegation(s.ctx, pos.Id))

	// 1. Position deleted.
	_, err = s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().Error(err, "position must be deleted after force exit")

	// 2. Delegation moves from position back to owner.
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().Error(err, "position delegator should no longer hold a delegation")
	ownerDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, owner, valAddr)
	s.Require().NoError(err, "owner must have staking delegation back")
	s.Require().True(ownerDel.Shares.IsPositive())
	s.Require().Equal(owner.String(), ownerDel.DelegatorAddress,
		"returned delegation must be keyed at the vesting owner address")

	// 3. DelegatedVesting unchanged — and now consistent with the actual
	// delegation, since the delegation is back at the owner.
	finalDV := s.delegatedVesting(owner)
	s.Require().Equal(initialDV.String(), finalDV.String(),
		"DelegatedVesting must remain unchanged across the round trip")

	// 4. Rewards arrived at the owner's bank balance and are spendable.
	balAfter := s.app.BankKeeper.GetBalance(s.ctx, owner, bondDenom).Amount
	rewardsClaimed := balAfter.Sub(balBefore)
	s.Require().True(rewardsClaimed.IsPositive(),
		"rewards must arrive at the owner's bank balance during force exit; got %s -> %s",
		balBefore, balAfter)

	spendable := s.app.BankKeeper.SpendableCoins(s.ctx, owner)
	s.Require().True(spendable.AmountOf(bondDenom).Equal(balAfter),
		"rewards (and the rest of the post-exit bank balance) must be fully spendable; "+
			"got spendable=%s, balance=%s",
		spendable.AmountOf(bondDenom), balAfter)
}
