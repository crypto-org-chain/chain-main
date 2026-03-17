package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"
)

func (s *KeeperSuite) TestInitExportGenesis_RoundTrip() {
	customParams := types.NewParams(
		sdkmath.LegacyNewDecWithPrec(3, 2), // 0.03
		nil,
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
		sdkmath.LegacyNewDecWithPrec(5, 2), // 0.05
		nil,
	)
	s.keeper.InitGenesis(s.ctx, &types.GenesisState{Params: original})

	exported1 := s.keeper.ExportGenesis(s.ctx)
	s.keeper.InitGenesis(s.ctx, exported1)
	exported2 := s.keeper.ExportGenesis(s.ctx)

	s.Require().True(exported1.Params.TargetBaseRewardsRate.Equal(exported2.Params.TargetBaseRewardsRate))
}
