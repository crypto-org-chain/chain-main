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

func (s *KeeperSuite) TestTransferDelegationToPool_SameValidator_PartialTransfer() {
	// Get the validator and delegator from genesis
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)

	// Get the genesis delegator (genAccs[0])
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().NotEmpty(dels)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	// Record state before transfer
	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Transfer half the tokens
	delTotalTokensBefore := valBefore.TokensFromShares(del.Shares).TruncateInt()
	delHalfTokens := delTotalTokensBefore.Quo(sdkmath.NewInt(2))
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: valAddr.String(),
		ValidatorDstAddress: valAddr.String(), // same validator
		Amount:              sdk.NewCoin(bondDenom, delHalfTokens),
	}

	newShares, err := s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().NoError(err)
	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	// Verify validator tokens unchanged (net-zero for same validator)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged for same-validator transfer")

	// Verify correct amount of tokens are transferred
	tokensTransferred := valAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delHalfTokens, tokensTransferred)

	// Verify source delegation reduced
	delAfter, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	delAfterTokens := valAfter.TokensFromShares(delAfter.Shares).TruncateInt()
	s.Require().True(delAfterTokens.Equal(delTotalTokensBefore.Sub(delHalfTokens)), "source delegation tokens should decrease by half")

	// Verify pool delegation created
	poolDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().NoError(err)
	poolDelTokens := valAfter.TokensFromShares(poolDel.Shares).TruncateInt()
	s.Require().True(poolDelTokens.Equal(delHalfTokens), "pool should have half the initial delegated tokens from user")

	// Verify no change in total tokens staked
	totalTokens := delAfterTokens.Add(poolDelTokens)
	s.Require().True(totalTokens.Equal(delTotalTokensBefore), "total tokens in the system should be equal to initial delegated tokens")
	s.Require().True(totalTokens.Equal(valAfter.Tokens), "total tokens should be equal to validator tokens")
}

func (s *KeeperSuite) TestTransferDelegationToPool_SameValidator_FullTransfer() {
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

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Get the full token amount for this delegation
	valBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	delTokensBefore := valBefore.TokensFromShares(del.Shares).TruncateInt()

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: valAddr.String(),
		ValidatorDstAddress: valAddr.String(),
		Amount:              sdk.NewCoin(bondDenom, delTokensBefore),
	}
	newShares, err := s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().NoError(err)
	valAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	// Verify validator tokens unchanged (net-zero for same validator)
	s.Require().NoError(err)
	s.Require().True(valBefore.Tokens.Equal(valAfter.Tokens), "validator tokens should be unchanged for same-validator transfer")

	// Verify correct amount of tokens are transferred
	tokensTransferred := valAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delTokensBefore, tokensTransferred)

	// Source delegation should be fully removed
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().Error(err, "source delegation should be removed after full transfer")

	// Verify pool delegation created
	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)
	poolDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, valAddr)
	s.Require().NoError(err)
	poolDelTokens := valAfter.TokensFromShares(poolDel.Shares).TruncateInt()
	s.Require().True(poolDelTokens.Equal(delTokensBefore), "pool should have initial delegation tokens")
	s.Require().True(poolDelTokens.Equal(valAfter.Tokens), "pool tokens should equal validator tokens")
}

func (s *KeeperSuite) TestTransferDelegationToPool_CrossValidator_PartialTransfer() {
	// Get the existing validator and delegator from genesis
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	srcVal := vals[0]
	srcValAddr, err := sdk.ValAddressFromBech32(srcVal.GetOperator())
	s.Require().NoError(err)

	// Get the genesis delegator
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, srcValAddr)
	s.Require().NoError(err)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Create a second validator using a new key
	destValPrivKey := secp256k1.GenPrivKey()
	destValPubKey := destValPrivKey.PubKey()
	destValAddr := sdk.ValAddress(destValPubKey.Address())

	description := stakingtypes.NewDescription("val2", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(
		sdkmath.LegacyNewDecWithPrec(10, 2), // 10%
		sdkmath.LegacyNewDecWithPrec(20, 2), // 20%
		sdkmath.LegacyNewDecWithPrec(1, 2),  // 1%
	)
	createValMsg, err := stakingtypes.NewMsgCreateValidator(
		destValAddr.String(),
		destValPubKey,
		sdk.NewCoin(bondDenom, sdkmath.NewInt(1000000)), // 1 token
		description,
		commission,
		sdkmath.OneInt(),
	)
	s.Require().NoError(err)

	// Fund the validator account so it can self-delegate
	destValAccAddr := sdk.AccAddress(destValPubKey.Address())
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(2000000)))
	err = s.app.BankKeeper.SendCoins(s.ctx, delAddr, destValAccAddr, coins)
	s.Require().NoError(err)

	// Create the second validator
	stakingServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	_, err = stakingServer.CreateValidator(s.ctx, createValMsg)
	s.Require().NoError(err)

	// Record state before transfer
	srcValBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, srcValAddr)
	s.Require().NoError(err)
	dstValBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, destValAddr)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)

	// Transfer half the delegation tokens from srcValidator to dstValidator
	delTokensBefore := srcValBefore.TokensFromShares(del.Shares).TruncateInt()
	halfTokens := delTokensBefore.Quo(sdkmath.NewInt(2))

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: srcValAddr.String(),
		ValidatorDstAddress: destValAddr.String(),
		Amount:              sdk.NewCoin(bondDenom, halfTokens),
	}
	newShares, err := s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().NoError(err)
	// Source validator tokens should decrease
	srcValAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, srcValAddr)
	s.Require().NoError(err)
	s.Require().True(srcValAfter.Tokens.LT(srcValBefore.Tokens),
		"source validator tokens should decrease")

	// Destination validator tokens should increase
	dstValAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, destValAddr)
	s.Require().NoError(err)
	s.Require().True(dstValAfter.Tokens.GT(dstValBefore.Tokens),
		"destination validator tokens should increase")

	// Verify correct amount of tokens are transferred
	tokensTransferred := dstValAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(tokensTransferred, halfTokens)

	// Source delegation still exists on the source validator
	delAfterSrc, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, srcValAddr)
	s.Require().NoError(err)

	// No user delegation on the destination validator (It should exist on the pool instead)
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, destValAddr)
	s.Require().Error(err, "destination delegation should be created only on the pool after partial transfer")
	delAfterSrcTokens := srcValAfter.TokensFromShares(delAfterSrc.Shares).TruncateInt()
	s.Require().True(delAfterSrcTokens.Equal(delTokensBefore.Sub(halfTokens)),
		"source delegation should be reduced by the transferred tokens")

	srcValidatorDecreaseInTokens := srcValBefore.Tokens.Sub(srcValAfter.Tokens)
	destValIncreaseInTokens := dstValAfter.Tokens.Sub(dstValBefore.Tokens)

	s.Require().True(srcValidatorDecreaseInTokens.Equal(destValIncreaseInTokens),
		"source validator decrease in tokens should equal destination validator increase in tokens")

	s.Require().True(srcValidatorDecreaseInTokens.Equal(halfTokens),
		"source validator decrease in tokens should equal half the transferred tokens")

	// Pool should have delegation to destination validator
	poolDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, destValAddr)
	s.Require().NoError(err)
	poolDelTokens := dstValAfter.TokensFromShares(poolDel.Shares).TruncateInt()
	s.Require().True(poolDelTokens.Equal(halfTokens),
		"pool should have half original delegated tokens from user to destination validator")

	// There should be no pool delegation on the source validator
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, srcValAddr)
	s.Require().Error(err, "pool delegation should be only exist on the destination validator")

	// Verify no change in total tokens staked
	totalTokensBefore := srcValBefore.Tokens.Add(dstValBefore.Tokens)
	totalTokensAfter := srcValAfter.Tokens.Add(dstValAfter.Tokens)
	s.Require().True(totalTokensBefore.Equal(totalTokensAfter), "total tokens in the system should not change")
}

func (s *KeeperSuite) TestTransferDelegationToPool_CrossValidator_FullTransfer() {
	// Get the existing validator and delegator from genesis
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	srcVal := vals[0]
	srcValAddr, err := sdk.ValAddressFromBech32(srcVal.GetOperator())
	s.Require().NoError(err)

	// Get the genesis delegator
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, srcValAddr)
	s.Require().NoError(err)
	del := dels[0]
	delAddr, err := s.app.AccountKeeper.AddressCodec().StringToBytes(del.DelegatorAddress)
	s.Require().NoError(err)

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Create a second validator using a new key
	destValPrivKey := secp256k1.GenPrivKey()
	destValPubKey := destValPrivKey.PubKey()
	destValAddr := sdk.ValAddress(destValPubKey.Address())

	description := stakingtypes.NewDescription("val2", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(
		sdkmath.LegacyNewDecWithPrec(10, 2), // 10%
		sdkmath.LegacyNewDecWithPrec(20, 2), // 20%
		sdkmath.LegacyNewDecWithPrec(1, 2),  // 1%
	)
	createValMsg, err := stakingtypes.NewMsgCreateValidator(
		destValAddr.String(),
		destValPubKey,
		sdk.NewCoin(bondDenom, sdkmath.NewInt(1000000)),
		description,
		commission,
		sdkmath.OneInt(),
	)
	s.Require().NoError(err)

	// Fund the validator account so it can self-delegate
	destValAccAddr := sdk.AccAddress(destValPubKey.Address())
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.NewInt(2000000)))
	err = s.app.BankKeeper.SendCoins(s.ctx, delAddr, destValAccAddr, coins)
	s.Require().NoError(err)

	// Create the second validator
	stakingServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	_, err = stakingServer.CreateValidator(s.ctx, createValMsg)
	s.Require().NoError(err)

	// Record state before transfer
	srcValBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, srcValAddr)
	s.Require().NoError(err)
	dstValBefore, err := s.app.StakingKeeper.GetValidator(s.ctx, destValAddr)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)

	// Transfer all delegation tokens from srcValidator to dstValidator
	delTokensBefore := srcValBefore.TokensFromShares(del.Shares).TruncateInt()

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: srcValAddr.String(),
		ValidatorDstAddress: destValAddr.String(),
		Amount:              sdk.NewCoin(bondDenom, delTokensBefore),
	}
	newShares, err := s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().NoError(err)

	srcValAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, srcValAddr)
	s.Require().NoError(err)
	dstValAfter, err := s.app.StakingKeeper.GetValidator(s.ctx, destValAddr)
	s.Require().NoError(err)

	// Source validator tokens should decrease by full delegation amount
	srcValidatorDecreaseInTokens := srcValBefore.Tokens.Sub(srcValAfter.Tokens)
	s.Require().True(srcValidatorDecreaseInTokens.Equal(delTokensBefore),
		"source validator should lose all delegated tokens")

	// Destination validator tokens should increase by same amount
	destValIncreaseInTokens := dstValAfter.Tokens.Sub(dstValBefore.Tokens)
	s.Require().True(srcValidatorDecreaseInTokens.Equal(destValIncreaseInTokens),
		"source validator decrease in tokens should equal destination validator increase in tokens")

	// Verify correct amount of tokens are transferred
	tokensTransferred := dstValAfter.TokensFromShares(newShares).TruncateInt()
	s.Require().Equal(delTokensBefore, tokensTransferred)

	// Source delegation should be fully removed
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, srcValAddr)
	s.Require().Error(err, "source delegation should be removed after full transfer")

	// No user delegation on the destination validator
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, destValAddr)
	s.Require().Error(err, "user should not have a delegation on the destination validator")

	// Pool should have delegation to destination validator with full amount
	poolDel, err := s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, destValAddr)
	s.Require().NoError(err)
	poolDelTokens := dstValAfter.TokensFromShares(poolDel.Shares).TruncateInt()
	s.Require().True(poolDelTokens.Equal(delTokensBefore),
		"pool should have all original delegated tokens on destination validator")

	// No pool delegation on the source validator
	_, err = s.app.StakingKeeper.GetDelegation(s.ctx, poolAddr, srcValAddr)
	s.Require().Error(err, "pool should not have a delegation on the source validator")

	// Verify no change in total tokens staked across both validators
	totalTokensBefore := srcValBefore.Tokens.Add(dstValBefore.Tokens)
	totalTokensAfter := srcValAfter.Tokens.Add(dstValAfter.Tokens)
	s.Require().True(totalTokensBefore.Equal(totalTokensAfter), "total tokens in the system should not change")
}

func (s *KeeperSuite) TestTransferDelegationToPool_ZeroAmount() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress([]byte("test_delegator_addr1")).String(),
		ValidatorSrcAddress: val.GetOperator(),
		ValidatorDstAddress: val.GetOperator(),
		Amount:              sdk.NewCoin(bondDenom, sdkmath.ZeroInt()),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrInvalidAmount)
}

func (s *KeeperSuite) TestTransferDelegationToPool_InvalidSrcValidator() {
	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress([]byte("test_delegator_addr1")).String(),
		ValidatorSrcAddress: sdk.ValAddress([]byte("nonexistent_val_addr")).String(),
		ValidatorDstAddress: sdk.ValAddress([]byte("nonexistent_val_addr")).String(),
		Amount:              sdk.NewCoin(bondDenom, sdkmath.NewInt(1000)),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrBadTransferDelegationSrc)
}

func (s *KeeperSuite) TestTransferDelegationToPool_InvalidDstValidator() {
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

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: valAddr.String(),
		ValidatorDstAddress: sdk.ValAddress([]byte("nonexistent_val_addr")).String(),
		Amount:              sdk.NewCoin(bondDenom, sdkmath.NewInt(1000)),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrBadTransferDelegationDest)
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

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	// Try to transfer more tokens than delegated
	validator, err := s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	excessTokens := validator.TokensFromShares(del.Shares).TruncateInt().Add(sdkmath.NewInt(1000000))

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: valAddr.String(),
		ValidatorDstAddress: valAddr.String(),
		Amount:              sdk.NewCoin(bondDenom, excessTokens),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidRequest)
}

func (s *KeeperSuite) TestTransferDelegationToPool_PoolCannotTransferToSelf() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	poolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    poolAddr.String(),
		ValidatorSrcAddress: val.GetOperator(),
		ValidatorDstAddress: val.GetOperator(),
		Amount:              sdk.NewCoin(bondDenom, sdkmath.NewInt(1000)),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrTransferDelegationToPoolSelf)
}

func (s *KeeperSuite) TestTransferDelegationToPool_NoDelegation() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	randomAddr := sdk.AccAddress([]byte("addr_with_no_delegation"))

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    randomAddr.String(),
		ValidatorSrcAddress: val.GetOperator(),
		ValidatorDstAddress: val.GetOperator(),
		Amount:              sdk.NewCoin(bondDenom, sdkmath.NewInt(1000)),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err, "should fail when delegator has no delegation")
}

func (s *KeeperSuite) TestTransferDelegationToPool_InvalidFromAddress() {
	vals, err := s.app.StakingKeeper.GetBondedValidatorsByPower(s.ctx)
	s.Require().NoError(err)
	val := vals[0]

	bondDenom, err := s.app.StakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    "invalid_address",
		ValidatorSrcAddress: val.GetOperator(),
		ValidatorDstAddress: val.GetOperator(),
		Amount:              sdk.NewCoin(bondDenom, sdkmath.NewInt(1000)),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
}

func (s *KeeperSuite) TestTransferDelegationToPool_WrongBondDenom() {
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

	msg := types.MsgCommitDelegationToTier{
		DelegatorAddress:    sdk.AccAddress(delAddr).String(),
		ValidatorSrcAddress: valAddr.String(),
		ValidatorDstAddress: valAddr.String(),
		Amount:              sdk.NewCoin("wrongdenom", sdkmath.NewInt(1000)),
	}
	_, err = s.keeper.TransferDelegationToPool(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, sdkerrors.ErrInvalidRequest)
}
