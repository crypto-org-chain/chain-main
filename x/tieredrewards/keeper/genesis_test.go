package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"
)

func (s *KeeperSuite) TestInitExportGenesis_RoundTrip() {
	customParams := types.NewParams(
		sdkmath.LegacyNewDecWithPrec(3, 2), []types.TierDefinition{}, []string{}, // 0.03
	)
	s.keeper.InitGenesis(s.ctx, &types.GenesisState{Params: customParams})

	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().True(customParams.TargetBaseRewardsRate.Equal(exported.Params.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestInitExportGenesis_DefaultParams() {
	defaultGenesis := types.DefaultGenesisState()
	s.keeper.InitGenesis(s.ctx, defaultGenesis)

	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().True(exported.Params.TargetBaseRewardsRate.IsZero())
}

func (s *KeeperSuite) TestInitExportGenesis_ReImport() {
	original := types.NewParams(
		sdkmath.LegacyNewDecWithPrec(5, 2), []types.TierDefinition{}, []string{}, // 0.05
	)
	s.keeper.InitGenesis(s.ctx, &types.GenesisState{Params: original})

	exported1 := s.keeper.ExportGenesis(s.ctx)
	s.keeper.InitGenesis(s.ctx, exported1)
	exported2 := s.keeper.ExportGenesis(s.ctx)

	s.Require().True(exported1.Params.TargetBaseRewardsRate.Equal(exported2.Params.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestInitExportGenesis_WithPositions() {
	positions := []types.TierPosition{
		{
			PositionId:      1,
			Owner:           "cosmos1abc",
			TierId:          1,
			AmountLocked:    sdkmath.NewInt(1000),
			DelegatedShares: sdkmath.LegacyZeroDec(),
		},
		{
			PositionId:      2,
			Owner:           "cosmos1def",
			TierId:          2,
			AmountLocked:    sdkmath.NewInt(5000),
			DelegatedShares: sdkmath.LegacyZeroDec(),
		},
	}

	genesis := &types.GenesisState{
		Params:         types.DefaultParams(),
		Positions:      positions,
		NextPositionId: 3,
	}
	s.keeper.InitGenesis(s.ctx, genesis)

	// Export and verify round-trip.
	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().Len(exported.Positions, 2)
	s.Require().Equal(uint64(3), exported.NextPositionId)

	// Verify each position.
	s.Require().Equal(uint64(1), exported.Positions[0].PositionId)
	s.Require().Equal("cosmos1abc", exported.Positions[0].Owner)
	s.Require().True(exported.Positions[0].AmountLocked.Equal(sdkmath.NewInt(1000)))

	s.Require().Equal(uint64(2), exported.Positions[1].PositionId)
	s.Require().Equal("cosmos1def", exported.Positions[1].Owner)
	s.Require().True(exported.Positions[1].AmountLocked.Equal(sdkmath.NewInt(5000)))

	// Re-import and verify again.
	s.keeper.InitGenesis(s.ctx, exported)
	exported2 := s.keeper.ExportGenesis(s.ctx)
	s.Require().Len(exported2.Positions, 2)
	s.Require().Equal(exported.NextPositionId, exported2.NextPositionId)
}
