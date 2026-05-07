package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// --- transferDelegationToPosition tests ---
func (s *KeeperSuite) TestTransferDelegationToPosition_PartialTransfer() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().NotEmpty(dels)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	delTotalTokensBefore := valBefore.TokensFromShares(del.Shares).TruncateInt()
	delHalfTokens := delTotalTokensBefore.Quo(sdkmath.NewInt(2))
	posDelAddr := types.GetDelegatorAddress(1)

	newShares, err := s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress(delAddr).String(), posDelAddr, valAddr.String(), delHalfTokens)
	s.Require().NoError(err)

	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged for same-validator transfer")

	tokensTransferred := valAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delHalfTokens, tokensTransferred)

	// Source delegation reduced.
	delAfter, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	delAfterTokens := valAfter.TokensFromShares(delAfter.Shares).TruncateInt()
	s.Require().True(delAfterTokens.Equal(delTotalTokensBefore.Sub(delHalfTokens)), "source delegation tokens should decrease by half")

	// Position's delegator address now holds the transferred delegation.
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	posDelTokens := valAfter.TokensFromShares(posDel.Shares).TruncateInt()
	s.Require().True(posDelTokens.Equal(delHalfTokens), "position's delegator address should hold half the initial delegated tokens")

	// Total tokens unchanged.
	totalTokens := delAfterTokens.Add(posDelTokens)
	s.Require().True(totalTokens.Equal(delTotalTokensBefore), "total tokens should equal initial delegation")
}

func (s *KeeperSuite) TestTransferDelegationToPosition_FullTransfer() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	delTokensBefore := valBefore.TokensFromShares(del.Shares).TruncateInt()
	posDelAddr := types.GetDelegatorAddress(1)

	newShares, err := s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress(delAddr).String(), posDelAddr, valAddr.String(), delTokensBefore)
	s.Require().NoError(err)

	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged for same-validator transfer")

	tokensTransferred := valAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delTokensBefore, tokensTransferred)

	// Source delegation fully removed.
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().Error(err, "source delegation should be removed after full transfer")

	// Position's delegator address holds the full delegation.
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	posDelTokens := valAfter.TokensFromShares(posDel.Shares).TruncateInt()
	s.Require().True(posDelTokens.Equal(delTokensBefore), "position's delegator address should hold all delegated tokens")
	s.Require().True(posDelTokens.Equal(valAfter.Tokens), "position delegator tokens should equal validator tokens")
}

func (s *KeeperSuite) TestTransferDelegationToPosition_ZeroAmount() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress([]byte("test_delegator_addr1")).String(), types.GetDelegatorAddress(1), val.GetOperator(), sdkmath.ZeroInt())
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidRequest)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_TinyAmount() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().NotEmpty(dels)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	validator, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// Force an exchange rate where 1 token maps to < 1e-18 shares, so
	// SharesFromTokens(1) truncates to zero shares and Unbond returns zero tokens.
	validator.DelegatorShares = sdkmath.LegacyOneDec()
	validator.Tokens = sdkmath.NewIntWithDecimal(1, 19)
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, validator))

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress(delAddr).String(), types.GetDelegatorAddress(1), valAddr.String(), sdkmath.OneInt())
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTinyTransferDelegationAmount)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_InvalidValidator() {
	_, err := s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress([]byte("test_delegator_addr1")).String(), types.GetDelegatorAddress(1), sdk.ValAddress([]byte("nonexistent_val_addr")).String(), sdkmath.NewInt(1000))
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTransferDelegationSrcNotFound)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_InsufficientTokens() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	validator, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	excessTokens := validator.TokensFromShares(del.Shares).TruncateInt().Add(sdkmath.NewInt(1000000))

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress(delAddr).String(), types.GetDelegatorAddress(1), valAddr.String(), excessTokens)
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidRequest)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_PositionCannotTransferToSelf() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	posDelAddr := types.GetDelegatorAddress(1)

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, posDelAddr.String(), posDelAddr, val.GetOperator(), sdkmath.NewInt(1000))
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTransferDelegationToPositionSelf)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_NoDelegation() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	randomAddr := sdk.AccAddress([]byte("addr_with_no_delegation"))

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, randomAddr.String(), types.GetDelegatorAddress(1), val.GetOperator(), sdkmath.NewInt(1000))
	s.Require().Error(err, "should fail when delegator has no delegation")
}

func (s *KeeperSuite) TestTransferDelegationToPosition_InvalidFromAddress() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	_, err = s.keeper.TransferDelegationToPosition(s.ctx, "invalid_address", types.GetDelegatorAddress(1), val.GetOperator(), sdkmath.NewInt(1000))
	s.Require().Error(err)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_RejectsActiveRedelegation() {
	// Get genesis validator (bonded) and delegator.
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	genesisValAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, genesisValAddr)
	s.Require().NoError(err)
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(dels[0].DelegatorAddress)
	s.Require().NoError(err)

	// Create second bonded validator.
	secondValAddr, _ := s.createSecondValidator()

	secondVal, err := s.app.StakingKeeper.GetValidator(s.ctx, secondValAddr)
	s.Require().NoError(err)
	s.Require().True(secondVal.IsBonded(), "second validator should be bonded after ApplyAndReturnValidatorSetUpdates")

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Redelegate from genesis validator (bonded source) to second validator (bonded dest).
	// Source is bonded so completeNow=false → redelegation entry IS created.
	stakingServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	redelMsg := stakingtypes.NewMsgBeginRedelegate(
		sdk.AccAddress(delAddr).String(),
		genesisValAddr.String(),
		secondValAddr.String(),
		sdk.NewCoin(bondDenom, sdkmath.NewInt(100_000)),
	)
	_, err = stakingServer.BeginRedelegate(s.ctx, redelMsg)
	s.Require().NoError(err)

	// Transfer at the second validator must be blocked to prevent slash escape.
	_, err = s.keeper.TransferDelegationToPosition(s.ctx, sdk.AccAddress(delAddr).String(), types.GetDelegatorAddress(1), secondValAddr.String(), sdkmath.NewInt(50_000))
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrActiveRedelegation)
}

func (s *KeeperSuite) TestTransferDelegationToPosition_RejectsUnbondedValidator() {
	// Create a second validator, then jail it so it goes to unbonding.
	dstValAddr, dstAccAddr := s.createSecondValidator()

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Delegate from dstAccAddr to dstValAddr so they have a delegation to transfer.
	stakingServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	delegateMsg := stakingtypes.NewMsgDelegate(
		dstAccAddr.String(),
		dstValAddr.String(),
		sdk.NewCoin(bondDenom, sdkmath.NewInt(100_000)),
	)
	_, err = stakingServer.Delegate(s.ctx, delegateMsg)
	s.Require().NoError(err)

	// Jail the validator — removes it from the power index.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	valConsAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Jail(s.ctx, valConsAddr)
	s.Require().NoError(err)

	// Apply validator set updates so the jailed validator transitions to Unbonding.
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	val, err = s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "jailed validator should not be bonded")

	// Transfer on the now-jailed (unbonding) validator must fail.
	_, err = s.keeper.TransferDelegationToPosition(s.ctx, dstAccAddr.String(), types.GetDelegatorAddress(1), dstValAddr.String(), sdkmath.NewInt(50_000))
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)
}

// --- transferDelegationFromPosition tests ---

func (s *KeeperSuite) TestTransferDelegationFromPosition_Basic() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)

	s.advancePastExitDuration()

	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Record validator tokens before transfer.
	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	// Compute token value from shares for the full amount.
	positionAmount := valBefore.TokensFromShares(pos.Delegation.Shares).TruncateInt()

	returnShares, _, _, err := s.keeper.TransferDelegationFromPosition(s.ctx, pos, valAddr, positionAmount)
	s.Require().NoError(err)
	s.Require().True(returnShares.IsPositive())

	// Validator tokens should be unchanged (same-validator transfer).
	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged")

	// Owner should have a staking delegation.
	ownerDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(pos.Delegation.Shares, ownerDel.Shares)

	// Position's delegation should be removed (all shares transferred).
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().Error(err, "position delegation should be removed after full transfer")
}

func (s *KeeperSuite) TestTransferDelegationFromPosition_Partial() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)

	s.advancePastExitDuration()

	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	posDelAddr := types.GetDelegatorAddress(pos.Id)

	// Compute token value from shares for the half amount.
	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	positionAmount := valBefore.TokensFromShares(pos.Delegation.Shares).TruncateInt()
	halfAmount := positionAmount.Quo(sdkmath.NewInt(2))

	returnShares, _, _, err := s.keeper.TransferDelegationFromPosition(s.ctx, pos, valAddr, halfAmount)
	s.Require().NoError(err)
	s.Require().True(returnShares.IsPositive())

	// Owner should have a staking delegation.
	ownerDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(ownerDel.Shares.IsPositive())

	// Position should still hold the remaining delegation.
	posDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, posDelAddr, valAddr)
	s.Require().NoError(err)
	s.Require().True(posDel.Shares.IsPositive(), "position should still have remaining delegation")
}

func (s *KeeperSuite) TestTransferDelegationFromPosition_ValidatorNotBonded() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)

	s.advancePastExitDuration()

	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	s.jailAndUnbondValidator(valAddr)

	// Compute token value from shares.
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	positionAmount := val.TokensFromShares(pos.Delegation.Shares).TruncateInt()

	_, _, _, err = s.keeper.TransferDelegationFromPosition(s.ctx, pos, valAddr, positionAmount)
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)
}

func (s *KeeperSuite) TestTransferDelegationFromPosition_InvalidOwnerAddress() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)

	s.advancePastExitDuration()

	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	positionAmount := val.TokensFromShares(pos.Delegation.Shares).TruncateInt()

	pos.Owner = "invalid_address"
	_, _, _, err = s.keeper.TransferDelegationFromPosition(s.ctx, pos, valAddr, positionAmount)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestTransferDelegationFromPosition_NotDelegated() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)
	s.advancePastExitDuration()

	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)

	// Undelegate so position is no longer delegated.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierUndelegate(s.ctx, &types.MsgTierUndelegate{
		Owner:      pos.Owner,
		PositionId: pos.Id,
	})
	s.Require().NoError(err)

	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().False(pos.IsDelegated())

	_, _, _, err = s.keeper.TransferDelegationFromPosition(s.ctx, pos, valAddr, s.getPositionAmount(pos))
	s.Require().ErrorIs(err, types.ErrPositionNotDelegated)
}

func (s *KeeperSuite) TestTransferDelegationFromPosition_ActiveRedelegation() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, true)
	_, bondDenom := s.getStakingData()
	s.fundRewardsPool(sdkmath.NewInt(1_000_000), bondDenom)

	// Redelegate BEFORE exit elapses (redelegate is blocked after exit elapsed).
	dstValAddr, _ := s.createSecondValidator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	_, err := msgServer.TierRedelegate(s.ctx, &types.MsgTierRedelegate{
		Owner:        pos.Owner,
		PositionId:   pos.Id,
		DstValidator: dstValAddr.String(),
	})
	s.Require().NoError(err)

	// Now advance past exit duration.
	s.advancePastExitDuration()

	// Re-fetch position after redelegate (validator changed).
	pos, err = s.keeper.GetPositionState(s.ctx, pos.Id)
	s.Require().NoError(err)

	newValAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, _, _, err = s.keeper.TransferDelegationFromPosition(s.ctx, pos, newValAddr, s.getPositionAmount(pos))
	s.Require().ErrorIs(err, types.ErrActiveRedelegation)
}

func (s *KeeperSuite) TestTransferDelegationFromPosition_OwnerHasExistingDelegation() {
	lockAmount := sdkmath.NewInt(10000)
	pos := s.setupNewTierPosition(lockAmount, false)

	ownerAddr := sdk.MustAccAddressFromBech32(pos.Owner)
	valAddr := sdk.MustValAddressFromBech32(pos.Delegation.ValidatorAddress)
	_, bondDenom := s.getStakingData()

	// Give the owner a personal delegation on the same validator.
	personalAmount := sdkmath.NewInt(5000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, ownerAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, personalAmount)))
	s.Require().NoError(err)

	val, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, ownerAddr, personalAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	// Record owner's delegation shares before transfer.
	delBefore, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)

	// Compute token value from shares for the full amount.
	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	positionAmount := val.TokensFromShares(pos.Delegation.Shares).TruncateInt()

	// Transfer tier delegation back to owner.
	returnShares, _, _, err := s.keeper.TransferDelegationFromPosition(s.ctx, pos, valAddr, positionAmount)
	s.Require().NoError(err)
	s.Require().True(returnShares.IsPositive())

	// Owner's delegation shares should have increased (added to existing).
	delAfter, err := s.app.StakingKeeper.GetDelegation(s.ctx, ownerAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(delBefore.Shares.Add(pos.Delegation.Shares), delAfter.Shares, "owner delegation shares should increase by the amount transferred")
}
