package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperSuite) TestTransferDelegation_PartialTransfer() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)

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
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress(delAddr).String(),
		ValidatorAddress: valAddr.String(),
		Amount:           delHalfTokens,
	}

	newShares, err := s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().NoError(err)

	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged for same-validator transfer")

	tokensTransferred := valAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delHalfTokens, tokensTransferred)

	// Source delegation reduced
	delAfter, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	delAfterTokens := valAfter.TokensFromShares(delAfter.Shares).TruncateInt()
	s.Require().True(delAfterTokens.Equal(delTotalTokensBefore.Sub(delHalfTokens)), "source delegation tokens should decrease by half")

	// Module delegation created on same validator
	poolDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().NoError(err)
	poolDelTokens := valAfter.TokensFromShares(poolDel.Shares).TruncateInt()
	s.Require().True(poolDelTokens.Equal(delHalfTokens), "module should have half the initial delegated tokens")

	// Total tokens unchanged
	totalTokens := delAfterTokens.Add(poolDelTokens)
	s.Require().True(totalTokens.Equal(delTotalTokensBefore), "total tokens should equal initial delegation")
}

func (s *KeeperSuite) TestTransferDelegation_FullTransfer() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	delTokensBefore := valBefore.TokensFromShares(del.Shares).TruncateInt()
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress(delAddr).String(),
		ValidatorAddress: valAddr.String(),
		Amount:           delTokensBefore,
	}

	newShares, err := s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().NoError(err)

	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged for same-validator transfer")

	tokensTransferred := valAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delTokensBefore, tokensTransferred)

	// Source delegation fully removed
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().Error(err, "source delegation should be removed after full transfer")

	// Module has full delegation
	poolDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().NoError(err)
	poolDelTokens := valAfter.TokensFromShares(poolDel.Shares).TruncateInt()
	s.Require().True(poolDelTokens.Equal(delTokensBefore), "module should have all delegated tokens")
	s.Require().True(poolDelTokens.Equal(valAfter.Tokens), "module tokens should equal validator tokens")
}

func (s *KeeperSuite) TestTransferDelegation_ZeroAmount() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress([]byte("test_delegator_addr1")).String(),
		ValidatorAddress: val.GetOperator(),
		Amount:           sdkmath.ZeroInt(),
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidRequest)
}

func (s *KeeperSuite) TestTransferDelegation_InvalidValidator() {
	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress([]byte("test_delegator_addr1")).String(),
		ValidatorAddress: sdk.ValAddress([]byte("nonexistent_val_addr")).String(),
		Amount:           sdkmath.NewInt(1000),
	}
	_, err := s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrBadTransferDelegationSrc)
}

func (s *KeeperSuite) TestTransferDelegation_InsufficientTokens() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	validator, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	excessTokens := validator.TokensFromShares(del.Shares).TruncateInt().Add(sdkmath.NewInt(1000000))

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress(delAddr).String(),
		ValidatorAddress: valAddr.String(),
		Amount:           excessTokens,
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidRequest)
}

func (s *KeeperSuite) TestTransferDelegation_PoolCannotTransferToSelf() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: poolAddr.String(),
		ValidatorAddress: val.GetOperator(),
		Amount:           sdkmath.NewInt(1000),
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTransferDelegationToPoolSelf)
}

func (s *KeeperSuite) TestTransferDelegation_NoDelegation() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	randomAddr := sdk.AccAddress([]byte("addr_with_no_delegation"))

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: randomAddr.String(),
		ValidatorAddress: val.GetOperator(),
		Amount:           sdkmath.NewInt(1000),
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err, "should fail when delegator has no delegation")
}

func (s *KeeperSuite) TestTransferDelegation_InvalidFromAddress() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: "invalid_address",
		ValidatorAddress: val.GetOperator(),
		Amount:           sdkmath.NewInt(1000),
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
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

	// Fund the validator account
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	srcValAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, srcValAddr)
	s.Require().NoError(err)
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(dels[0].DelegatorAddress)
	s.Require().NoError(err)
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(2_000_000)))
	err = s.app.BankKeeper.SendCoins(s.ctx, delAddr, accAddr, coins)
	s.Require().NoError(err)

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

func (s *KeeperSuite) TestTransferDelegation_RejectsActiveRedelegation() {
	// Get genesis validator (bonded) and delegator
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	genesisValAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
	s.Require().NoError(err)

	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, genesisValAddr)
	s.Require().NoError(err)
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(dels[0].DelegatorAddress)
	s.Require().NoError(err)

	// Create second bonded validator
	secondValAddr, _ := s.createSecondValidator()

	// Verify second validator is bonded
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

	// Try to transfer delegation at the second validator (bonded, but has active incoming redelegation).
	// Should be blocked to prevent slash escape.
	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress(delAddr).String(),
		ValidatorAddress: secondValAddr.String(),
		Amount:           sdkmath.NewInt(50_000),
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrActiveRedelegation)
}

func (s *KeeperSuite) TestTransferDelegation_RejectsUnbondedValidator() {
	// Create a second validator, then jail it so it goes to unbonding
	dstValAddr, dstAccAddr := s.createSecondValidator()

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Delegate from dstAccAddr to dstValAddr so they have a delegation to transfer
	stakingServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	delegateMsg := stakingtypes.NewMsgDelegate(
		dstAccAddr.String(),
		dstValAddr.String(),
		sdk.NewCoin(bondDenom, sdkmath.NewInt(100_000)),
	)
	_, err = stakingServer.Delegate(s.ctx, delegateMsg)
	s.Require().NoError(err)

	// Jail the validator — removes it from the power index
	val, err := s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	valConsAddr, err := val.GetConsAddr()
	s.Require().NoError(err)
	err = s.app.StakingKeeper.Jail(s.ctx, valConsAddr)
	s.Require().NoError(err)

	// Apply validator set updates so the jailed validator transitions to Unbonding
	_, err = s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(s.ctx)
	s.Require().NoError(err)

	// Verify validator is no longer bonded
	val, err = s.app.StakingKeeper.GetValidator(s.ctx, dstValAddr)
	s.Require().NoError(err)
	s.Require().False(val.IsBonded(), "jailed validator should not be bonded")

	// Try to transfer delegation on the now-jailed (unbonding) validator
	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: dstAccAddr.String(),
		ValidatorAddress: dstValAddr.String(),
		Amount:           sdkmath.NewInt(50_000),
	}
	_, err = s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrValidatorNotBonded)
}
