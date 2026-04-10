package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// ---------------------------------------------------------------------------
// slashRedelegationPosition tests (AfterRedelegationSlashed)
// ---------------------------------------------------------------------------

// Redelegation slash reduces both Amount and DelegatedShares.
func (s *KeeperSuite) TestSlashRedelegationPosition_ReducesBoth() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()

	lockAmount := sdkmath.NewInt(10000)
	const unbondingId uint64 = 42

	_, pos := s.setupDelegatedPositionForSlash(valAddr, bondDenom, lockAmount, unbondingId, true)
	origShares := pos.DelegatedShares
	s.Require().True(origShares.IsPositive())

	// Use 10% of actual shares so the position stays delegated after slash.
	slashTokens := sdkmath.NewInt(1000)
	shareBurnt := origShares.Quo(sdkmath.LegacyNewDec(10))

	err := s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.Amount.Equal(lockAmount.Sub(slashTokens)),
		"Amount should be reduced; got %s, want %s", updated.Amount, lockAmount.Sub(slashTokens))
	s.Require().True(updated.DelegatedShares.Equal(origShares.Sub(shareBurnt)),
		"DelegatedShares should be reduced; got %s, want %s", updated.DelegatedShares, origShares.Sub(shareBurnt))
	s.Require().True(updated.IsDelegated(), "position should still be delegated")
}

// When all shares are burnt, the position should clear its delegation and set amount to zero.
func (s *KeeperSuite) TestSlashRedelegationPosition_AllSharesBurnt() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()

	lockAmount := sdkmath.NewInt(5000)
	const unbondingId uint64 = 43

	_, pos := s.setupDelegatedPositionForSlash(valAddr, bondDenom, lockAmount, unbondingId, true)
	origShares := pos.DelegatedShares

	// all tokens should be slashed if all shares are burnt
	slashTokens := lockAmount
	shareBurnt := origShares.Add(sdkmath.LegacyOneDec())

	err := s.keeper.Hooks().AfterRedelegationSlashed(s.ctx, unbondingId, slashTokens, shareBurnt)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().False(updated.IsDelegated(),
		"position should have cleared delegation when shareBurnt exceeds shares")
	s.Require().True(updated.DelegatedShares.IsZero())
	s.Require().True(updated.Amount.IsZero(),
		"Amount should be zero when all shares are destroyed")
}

// Unknown unbondingId is a no-op (non-tier delegation).
func (s *KeeperSuite) TestSlashRedelegationPosition_UnknownId() {
	s.setupTierAndDelegator()

	err := s.keeper.Hooks().AfterRedelegationSlashed(
		s.ctx, 999, sdkmath.NewInt(100), sdkmath.LegacyNewDec(50))
	s.Require().NoError(err) // no-op, no error
}

// Unbonding delegation slash reduces Amount but keeps DelegatedShares unchanged.
func (s *KeeperSuite) TestSlashUnbondingDelegationPosition_ReducesAmountOnly() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()

	lockAmount := sdkmath.NewInt(6000)
	const unbondingId uint64 = 45

	_, pos := s.setupDelegatedPositionForSlash(valAddr, bondDenom, lockAmount, unbondingId, false)
	origShares := pos.DelegatedShares
	slashTokens := sdkmath.NewInt(900)

	err := s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, unbondingId, slashTokens)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)

	s.Require().True(updated.Amount.Equal(lockAmount.Sub(slashTokens)))
	s.Require().True(updated.DelegatedShares.Equal(origShares),
		"DelegatedShares should not change for unbonding slash callbacks")
}

// Unbonding redelegation slash floors Amount at zero when slash exceeds Amount.
func (s *KeeperSuite) TestSlashUnbondingRedelegationPosition_FloorsAtZero() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()

	lockAmount := sdkmath.NewInt(4000)
	const unbondingId uint64 = 46

	_, pos := s.setupDelegatedPositionForSlash(valAddr, bondDenom, lockAmount, unbondingId, false)

	err := s.keeper.Hooks().AfterUnbondingRedelegationSlashed(s.ctx, unbondingId, sdkmath.NewInt(999999))
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, pos.Id)
	s.Require().NoError(err)
	s.Require().True(updated.Amount.IsZero(), "Amount should floor at zero when slash exceeds position amount")
}

// Unknown unbonding IDs should be no-op for both unbonding slash callbacks.
func (s *KeeperSuite) TestSlashUnbondingPosition_UnknownIdNoOp() {
	s.setupTierAndDelegator()

	err := s.keeper.Hooks().AfterUnbondingDelegationSlashed(s.ctx, 999, sdkmath.NewInt(100))
	s.Require().NoError(err)

	err = s.keeper.Hooks().AfterUnbondingRedelegationSlashed(s.ctx, 1000, sdkmath.NewInt(200))
	s.Require().NoError(err)
}

// ---------------------------------------------------------------------------
// Bonded slash (BeforeValidatorSlashed) regression — DelegatedShares must NOT change.
// ---------------------------------------------------------------------------

func (s *KeeperSuite) TestBondedSlash_DelegatedSharesUnchanged() {
	_, valAddr, bondDenom := s.setupTierAndDelegator()
	msgServer := keeper.NewMsgServerImpl(s.keeper)

	addr := sdk.AccAddress([]byte("bonded_slash_addr___"))
	lockAmount := sdkmath.NewInt(10000)
	err := banktestutil.FundAccount(s.ctx, s.app.BankKeeper, addr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount)))
	s.Require().NoError(err)

	_, err = msgServer.LockTier(s.ctx, &types.MsgLockTier{
		Owner:            addr.String(),
		Id:               1,
		Amount:           lockAmount,
		ValidatorAddress: valAddr.String(),
	})
	s.Require().NoError(err)

	positions, err := s.keeper.GetPositionsByOwner(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Len(positions, 1)
	origShares := positions[0].DelegatedShares

	fraction := sdkmath.LegacyNewDecWithPrec(1, 1) // 10%
	err = s.keeper.Hooks().BeforeValidatorSlashed(s.ctx, valAddr, fraction)
	s.Require().NoError(err)

	updated, err := s.keeper.GetPosition(s.ctx, positions[0].Id)
	s.Require().NoError(err)

	s.Require().True(updated.Amount.LT(lockAmount),
		"Amount should be reduced after bonded slash")
	s.Require().True(updated.DelegatedShares.Equal(origShares),
		"DelegatedShares must NOT change on bonded slash; got %s, want %s",
		updated.DelegatedShares, origShares)
	s.Require().True(updated.IsDelegated())
}
