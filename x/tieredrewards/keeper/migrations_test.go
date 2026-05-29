package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	migration "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/migrations/v2"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"
)

func (s *KeeperSuite) TestMigrate1to2_BackfillsPositionsWithDelegatorAddress() {
	s.setupTier(1)

	owner := s.fundRandomAddr("stake", sdkmath.NewInt(0))
	now := s.ctx.BlockTime()

	// Seed positions with empty DelegatorAddress to simulate v1 state.
	for _, id := range []uint64{1, 2, 5} {
		pos := types.NewPosition(id, owner.String(), 1, "", 100, 0, now, true, now)
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
