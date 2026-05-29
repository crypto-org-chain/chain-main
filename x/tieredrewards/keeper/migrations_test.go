package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	migration "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/migrations/v2"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

func (s *KeeperSuite) TestMigrate1to2_BackfillsPositionsWithDelegatorAddress() {
	s.setupTier(1)

	now := s.ctx.BlockTime()

	ownerAcc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, testutil.TestOwner)
	s.app.AccountKeeper.SetAccount(s.ctx, ownerAcc)

	// Seed positions with empty DelegatorAddress to simulate v1 state.
	for _, id := range []uint64{1, 2, 5} {
		pos := types.NewPosition(id, testutil.TestOwner.String(), 1, "", 100, 0, now, true, now)
		s.Require().NoError(s.keeper.Positions.Set(s.ctx, id, pos))
	}

	migrator := keeper.NewMigrator(s.keeper)
	s.Require().NoError(migrator.Migrate1to2(s.ctx))

	for _, id := range []uint64{1, 2, 5} {
		got, err := s.keeper.Positions.Get(s.ctx, id)
		s.Require().NoError(err)
		s.Require().Equal(migration.LegacyDelegatorAddress(id), got.DelegatorAddress,
			"position %d must be backfilled to legacy v1 address", id)
	}
}

func (s *KeeperSuite) TestMigrate1to2_ExitsVestedOwnerPositions() {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	amount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())

	// Regular (non-vesting) owner with a tier position; must survive.
	regularOwner := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	regularAcc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, regularOwner)
	s.app.AccountKeeper.SetAccount(s.ctx, regularAcc)
	s.Require().NoError(banktestutil.FundAccount(
		s.ctx, s.app.BankKeeper, regularOwner, sdk.NewCoins(sdk.NewCoin(bondDenom, amount)),
	))
	regularPos := s.createLockTierPositionV1(regularOwner, valAddr, amount)

	// Vesting owner with two tier positions.
	// Both must be deleted by the migration.
	vestingOwner := s.newVestingOwnerWithBalance(bondDenom, amount, amount.MulRaw(3))
	commitPos := s.createCommitPositionV1(vestingOwner, val, valAddr, amount)
	lockPos := s.createLockTierPositionV1(vestingOwner, valAddr, amount)

	s.advanceForRewards(valAddr, bondDenom)

	migrator := keeper.NewMigrator(s.keeper)
	s.Require().NoError(migrator.Migrate1to2(s.ctx))

	// Both vesting-owned positions deleted.
	_, err := s.keeper.Positions.Get(s.ctx, commitPos.Id)
	s.Require().Error(err, "commit-origin vesting position must be deleted")
	_, err = s.keeper.Positions.Get(s.ctx, lockPos.Id)
	s.Require().Error(err, "lock-origin vesting position must be deleted")

	// Regular position survives, with DelegatorAddress equal to the legacy
	// derivation.
	survived, err := s.keeper.Positions.Get(s.ctx, regularPos.Id)
	s.Require().NoError(err, "regular position must survive")
	s.Require().Equal(regularOwner.String(), survived.Owner)
	s.Require().Equal(migration.LegacyDelegatorAddress(regularPos.Id), survived.DelegatorAddress)
}
