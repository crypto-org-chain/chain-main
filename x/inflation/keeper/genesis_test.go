package keeper_test

import (
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"

	sdkmath "cosmossdk.io/math"
)

func (s *KeeperSuite) TestInitExportGenesis_RoundTrip() {
	// Set custom params
	customParams := types.NewParams(
		sdkmath.NewInt(999_999_000),
		[]string{"cosmos139f7kncmglres2nf3h4hc4tade85ekfr8sulz5"},
		sdkmath.LegacyNewDecWithPrec(68, 3), // 0.068
	)
	s.keeper.InitGenesis(s.ctx, types.GenesisState{
		Params:          customParams,
		DecayEpochStart: 1,
	})

	// Export and verify round-trip
	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().Equal(customParams.MaxSupply, exported.Params.MaxSupply)
	s.Require().Equal(customParams.BurnedAddresses, exported.Params.BurnedAddresses)
	s.Require().True(customParams.DecayRate.Equal(exported.Params.DecayRate))
	s.Require().Equal(uint64(1), exported.DecayEpochStart)
}

func (s *KeeperSuite) TestInitExportGenesis_DefaultParams() {
	// Init with default genesis
	defaultGenesis := *types.DefaultGenesis()
	s.keeper.InitGenesis(s.ctx, defaultGenesis)

	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().True(exported.Params.MaxSupply.IsZero())
	s.Require().Empty(exported.Params.BurnedAddresses)
	s.Require().True(types.DefaultParams().DecayRate.Equal(exported.Params.DecayRate))
	s.Require().Zero(exported.DecayEpochStart)
}

func (s *KeeperSuite) TestInitExportGenesis_ReImport() {
	// Set params, export, re-init, export again — must be identical
	original := types.NewParams(
		sdkmath.NewInt(500_000_000),
		[]string{
			"cosmos1dej28rxfh39axghzlcusd98qhpkdarcqqu23ua",
			"cosmos1g69pjvgvdug5m9kphwh284rvls4g5jnrg4p8dm",
		},
		sdkmath.LegacyNewDecWithPrec(5, 2), // 0.05
	)
	s.keeper.InitGenesis(s.ctx, types.GenesisState{
		Params:          original,
		DecayEpochStart: 1,
	})

	exported1 := s.keeper.ExportGenesis(s.ctx)
	s.keeper.InitGenesis(s.ctx, *exported1)
	exported2 := s.keeper.ExportGenesis(s.ctx)

	s.Require().Equal(exported1.Params.MaxSupply, exported2.Params.MaxSupply)
	s.Require().Equal(exported1.Params.BurnedAddresses, exported2.Params.BurnedAddresses)
	s.Require().True(exported1.Params.DecayRate.Equal(exported2.Params.DecayRate))
	s.Require().Equal(exported1.DecayEpochStart, exported2.DecayEpochStart)
}
