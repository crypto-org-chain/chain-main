package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// fundRewardsPool funds the rewards pool with the given amount.
func (s *KeeperSuite) fundRewardsPool(amount sdkmath.Int, denom string) {
	s.T().Helper()
	coins := sdk.NewCoins(sdk.NewCoin(denom, amount))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, types.RewardsPoolName, coins)
	s.Require().NoError(err)
}

// setupTier creates a new tier with a given id
func (s *KeeperSuite) setupTier(id uint32) {
	s.T().Helper()
	s.ctx = s.ctx.WithBlockHeight(1)
	s.ctx = s.ctx.WithBlockTime(time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC))

	tier := newTestTier(id)
	s.Require().NoError(s.keeper.SetTier(s.ctx, tier))
}

// getStakingData returns the bonded validators and bond denom
func (s *KeeperSuite) getStakingData() ([]stakingtypes.Validator, string) {
	s.T().Helper()
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	return vals, bondDenom
}

// setupNewTierPosition creates a new tier position with the given lock amount and funds the rewards pool.
func (s *KeeperSuite) setupNewTierPosition(lockAmount sdkmath.Int, triggerExitImmediately bool) types.Position {
	s.T().Helper()
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	freshAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, freshAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  freshAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: triggerExitImmediately,
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, freshAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	return positions[0]
}

// setupNewTierPositionWithDelegator creates a new tier position with an address who has already delegated to a validator.
func (s *KeeperSuite) setupNewTierPositionWithDelegator(lockAmount sdkmath.Int, triggerExitImmediately bool) types.Position {
	s.T().Helper()
	s.setupTier(1)
	_, bondDenom := s.getStakingData()
	delAddr, valAddr := s.getDelegator()

	msgServer := keeper.NewMsgServerImpl(s.keeper)

	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:                  delAddr.String(),
		Id:                     1,
		Amount:                 lockAmount,
		ValidatorAddress:       valAddr.String(),
		TriggerExitImmediately: triggerExitImmediately,
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, delAddr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)

	return positions[0]
}

// jailAndUnbondValidator jails a validator and runs ApplyAndReturnValidatorSetUpdates
// so the validator actually transitions to unbonding (which fires the hooks).
func (s *KeeperSuite) fundRandomAddr(denom string, amount sdkmath.Int) sdk.AccAddress {
	s.T().Helper()
	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr,
		sdk.NewCoins(sdk.NewCoin(denom, amount)))
	s.Require().NoError(err)
	return addr
}

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

// allocateRewardsToValidator funds the distribution module and allocates
// rewards to a validator so that WithdrawDelegationRewards returns them.
func (s *KeeperSuite) allocateRewardsToValidator(valAddr sdk.ValAddress, amount sdkmath.Int, denom string) {
	s.T().Helper()

	// Fund the distribution module account so it can back the allocation.
	rewardCoins := sdk.NewCoins(sdk.NewCoin(denom, amount))
	err := banktestutil.FundModuleAccount(s.ctx, s.app.BankKeeper, distrtypes.ModuleName, rewardCoins)
	s.Require().NoError(err)

	// Allocate through distribution so the rewards show up in WithdrawDelegationRewards.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	decRewards := sdk.NewDecCoinsFromCoins(rewardCoins...)
	err = s.app.DistrKeeper.AllocateTokensToValidator(s.ctx, val, decRewards)
	s.Require().NoError(err)
}

// getDelegator returns the genesis delegator address and validator address.
func (s *KeeperSuite) getDelegator() (sdk.AccAddress, sdk.ValAddress) {
	s.T().Helper()
	vals, _ := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().NotEmpty(dels)
	delAddrBytes, err := s.app.AccountKeeper.AddressCodec().StringToBytes(dels[0].DelegatorAddress)
	s.Require().NoError(err)
	return sdk.AccAddress(delAddrBytes), valAddr
}

// setValidatorCommission overrides the genesis validator's commission rate.
// The default genesis validator has 100% commission, which means delegators
// receive nothing from AllocateTokensToValidator. This helper sets it to
// a usable rate for reward tests.
func (s *KeeperSuite) setValidatorCommission(valAddr sdk.ValAddress, rate sdkmath.LegacyDec) {
	s.T().Helper()
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	val.Commission = stakingtypes.NewCommission(rate, sdkmath.LegacyOneDec(), sdkmath.LegacyZeroDec())
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, val))
}

// completeStakingUnbonding advances block time past the staking unbonding
// period and calls CompleteUnbonding so that the staking module returns
// matured tokens to the position's delegator address.
func (s *KeeperSuite) completeStakingUnbonding(valAddr sdk.ValAddress, delAddr sdk.AccAddress) {
	s.T().Helper()
	unbondingTime, err := s.app.StakingKeeper.UnbondingTime(s.ctx)
	s.Require().NoError(err)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(unbondingTime + time.Second))
	_, err = s.app.StakingKeeper.CompleteUnbonding(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
}

// slashUnbondingEntry mirrors what cosmos-sdk's slashing path does to a
// position's unbonding delegation: reduce the UD entry's Balance and fire the
// tier hook so internal accounting stays aligned with staking state.
func (s *KeeperSuite) slashUnbondingEntry(delAddr sdk.AccAddress, valAddr sdk.ValAddress, unbondingID uint64, slashAmount sdkmath.Int) {
	s.T().Helper()
	ubd, err := s.app.StakingKeeper.GetUnbondingDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	found := false
	for i, entry := range ubd.Entries {
		if entry.UnbondingId == unbondingID {
			ubd.Entries[i].Balance = entry.Balance.Sub(slashAmount)
			found = true
			break
		}
	}
	s.Require().True(found, "unbonding entry %d not found", unbondingID)
	s.Require().NoError(s.app.StakingKeeper.SetUnbondingDelegation(s.ctx, ubd))
	s.Require().NoError(s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, unbondingID, slashAmount))
}

// advancePastExitDuration advances block time past the default test tier's exit duration.
func (s *KeeperSuite) advancePastExitDuration() {
	s.T().Helper()
	tier, err := s.keeper.GetTier(s.ctx, 1)
	s.Require().NoError(err)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(tier.ExitDuration + time.Hour))
}

// slashValidatorDirect slashes a bonded validator through the staking module,
// changing the token/share exchange rate to non-1:1.
func (s *KeeperSuite) slashValidatorDirect(valAddr sdk.ValAddress, fraction sdkmath.LegacyDec) {
	s.T().Helper()
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	power := val.GetConsensusPower(s.app.StakingKeeper.PowerReduction(s.ctx))
	_, err = s.app.StakingKeeper.Slash(s.ctx, consAddr, s.ctx.BlockHeight(), power, fraction)
	s.Require().NoError(err)
}

// createSecondValidator creates a second bonded validator for tests that need
// cross-validator scenarios (redelegation, etc.)
func (s *KeeperSuite) createSecondValidator() (sdk.ValAddress, sdk.AccAddress) {
	s.T().Helper()

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey()
	valAddr := sdk.ValAddress(pubKey.Address())
	accAddr := sdk.AccAddress(pubKey.Address())

	// Fund the new validator from a source-validator delegator with spendable balance.
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	srcValAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, srcValAddr)
	s.Require().NoError(err)
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(2_000_000)))
	var funded bool
	for _, del := range dels {
		delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
		s.Require().NoError(err)
		spendable := s.app.BankKeeper.SpendableCoins(s.ctx, delAddr)
		if !spendable.IsAllGTE(coins) {
			continue
		}
		err = s.app.BankKeeper.SendCoins(s.ctx, delAddr, accAddr, coins)
		s.Require().NoError(err)
		funded = true
		break
	}
	s.Require().True(funded, "expected at least one existing delegator with enough spendable balance to fund validator creation")

	// Create validator
	description := stakingtypes.NewDescription("val2", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(
		sdkmath.LegacyNewDecWithPrec(10, 2),
		sdkmath.LegacyNewDecWithPrec(20, 2),
		sdkmath.LegacyNewDecWithPrec(1, 2),
	)
	createMsg, err := stakingtypes.NewMsgCreateValidator(
		valAddr.String(), pubKey,
		sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)),
		description, commission, sdkmath.OneInt(),
	)
	s.Require().NoError(err)

	stakingServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	_, err = stakingServer.CreateValidator(s.ctx, createMsg)
	s.Require().NoError(err)

	// Force the new validator into the bonded set
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	return valAddr, accAddr
}
