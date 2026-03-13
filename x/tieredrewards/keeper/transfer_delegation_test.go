package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (s *KeeperSuite) TestTransferDelegationToPool_PartialTransfer() {
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

func (s *KeeperSuite) TestTransferDelegationToPool_FullTransfer() {
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

func (s *KeeperSuite) TestTransferDelegationToPool_ZeroAmount() {
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

func (s *KeeperSuite) TestTransferDelegationToPool_InvalidValidator() {
	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress: sdk.AccAddress([]byte("test_delegator_addr1")).String(),
		ValidatorAddress: sdk.ValAddress([]byte("nonexistent_val_addr")).String(),
		Amount:           sdkmath.NewInt(1000),
	}
	_, err := s.keeper.TransferDelegation(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrBadTransferDelegationSrc)
}

func (s *KeeperSuite) TestTransferDelegationToPool_InsufficientTokens() {
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

func (s *KeeperSuite) TestTransferDelegationToPool_PoolCannotTransferToSelf() {
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

func (s *KeeperSuite) TestTransferDelegationToPool_NoDelegation() {
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

func (s *KeeperSuite) TestTransferDelegationToPool_InvalidFromAddress() {
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
