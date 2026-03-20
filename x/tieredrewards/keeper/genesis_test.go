package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperSuite) TestInitExportGenesis_RoundTrip() {
	customParams := types.NewParams(
		sdkmath.LegacyNewDecWithPrec(3, 2), // 0.03
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
	)
	s.keeper.InitGenesis(s.ctx, &types.GenesisState{Params: original})

	exported1 := s.keeper.ExportGenesis(s.ctx)
	s.keeper.InitGenesis(s.ctx, exported1)
	exported2 := s.keeper.ExportGenesis(s.ctx)

	s.Require().True(exported1.Params.TargetBaseRewardsRate.Equal(exported2.Params.TargetBaseRewardsRate))
}

func (s *KeeperSuite) TestInitExportGenesis_FullRoundTrip() {
	owner := sdk.AccAddress([]byte("genesis_test_owner__")).String()
	valAddr := sdk.ValAddress([]byte("genesis_test_val____"))
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build a full genesis state.
	tier1 := types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24 * 365,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(1000),
	}
	tier2 := types.Tier{
		Id:            2,
		ExitDuration:  time.Hour * 24 * 30,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(2, 2),
		MinLockAmount: sdkmath.NewInt(500),
		CloseOnly:     true,
	}

	// Non-delegated position.
	pos1 := types.Position{
		Id:              1,
		Owner:           owner,
		TierId:          1,
		Amount:          sdkmath.NewInt(5000),
		DelegatedShares: sdkmath.LegacyZeroDec(),
		CreatedAtHeight: 100,
		CreatedAtTime:   now,
	}

	// Delegated position.
	pos2 := types.Position{
		Id:                  2,
		Owner:               owner,
		TierId:              2,
		Amount:              sdkmath.NewInt(3000),
		Validator:           valAddr.String(),
		DelegatedShares:     sdkmath.LegacyNewDec(3000),
		BaseRewardsPerShare: sdk.NewDecCoins(sdk.NewDecCoinFromDec("stake", sdkmath.LegacyNewDecWithPrec(1, 4))),
		LastBonusAccrual:    now,
		CreatedAtHeight:     101,
		CreatedAtTime:       now,
	}

	// Position with exit triggered.
	exitTime := now.Add(-time.Hour * 24)
	pos3 := types.Position{
		Id:              3,
		Owner:           owner,
		TierId:          1,
		Amount:          sdkmath.NewInt(2000),
		DelegatedShares: sdkmath.LegacyZeroDec(),
		ExitTriggeredAt: exitTime,
		ExitUnlockAt:    exitTime.Add(time.Hour * 24 * 365),
		CreatedAtHeight: 99,
		CreatedAtTime:   now.Add(-time.Hour * 48),
	}

	genesisState := &types.GenesisState{
		Params:         types.NewParams(sdkmath.LegacyNewDecWithPrec(5, 2)),
		Tiers:          []types.Tier{tier1, tier2},
		Positions:      []types.Position{pos1, pos2, pos3},
		NextPositionId: 4,
		ValidatorRewardRatios: []types.ValidatorRewardRatioEntry{
			{
				Validator: valAddr.String(),
				RewardRatio: types.ValidatorRewardRatio{
					CumulativeRewardsPerShare: sdk.NewDecCoins(
						sdk.NewDecCoinFromDec("stake", sdkmath.LegacyNewDecWithPrec(5, 4)),
					),
				},
			},
		},
		UnbondingMappings: []types.UnbondingMapping{
			{UnbondingId: 42, PositionId: 2},
			{UnbondingId: 43, PositionId: 3},
		},
	}

	// Import genesis.
	s.keeper.InitGenesis(s.ctx, genesisState)

	// Export and compare.
	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)

	// Params.
	s.Require().True(genesisState.Params.TargetBaseRewardsRate.Equal(exported.Params.TargetBaseRewardsRate))

	// Tiers.
	s.Require().Len(exported.Tiers, 2)
	for i, tier := range exported.Tiers {
		s.Require().Equal(genesisState.Tiers[i].Id, tier.Id, "tier ID mismatch at %d", i)
		s.Require().Equal(genesisState.Tiers[i].ExitDuration, tier.ExitDuration)
		s.Require().True(genesisState.Tiers[i].BonusApy.Equal(tier.BonusApy))
		s.Require().True(genesisState.Tiers[i].MinLockAmount.Equal(tier.MinLockAmount))
		s.Require().Equal(genesisState.Tiers[i].CloseOnly, tier.CloseOnly)
	}

	// Positions.
	s.Require().Len(exported.Positions, 3)
	for i, pos := range exported.Positions {
		orig := genesisState.Positions[i]
		s.Require().Equal(orig.Id, pos.Id, "position ID mismatch at %d", i)
		s.Require().Equal(orig.Owner, pos.Owner)
		s.Require().Equal(orig.TierId, pos.TierId)
		s.Require().True(orig.Amount.Equal(pos.Amount))
		s.Require().Equal(orig.Validator, pos.Validator)
		s.Require().True(orig.DelegatedShares.Equal(pos.DelegatedShares))
		s.Require().Equal(orig.CreatedAtHeight, pos.CreatedAtHeight)
	}

	// Sequence.
	s.Require().Equal(genesisState.NextPositionId, exported.NextPositionId)

	// Validator reward ratios.
	s.Require().Len(exported.ValidatorRewardRatios, 1)
	s.Require().Equal(genesisState.ValidatorRewardRatios[0].Validator, exported.ValidatorRewardRatios[0].Validator)
	s.Require().True(genesisState.ValidatorRewardRatios[0].RewardRatio.CumulativeRewardsPerShare.Equal(
		exported.ValidatorRewardRatios[0].RewardRatio.CumulativeRewardsPerShare))

	// Unbonding mappings.
	s.Require().Len(exported.UnbondingMappings, 2)
	for i, m := range exported.UnbondingMappings {
		s.Require().Equal(genesisState.UnbondingMappings[i].UnbondingId, m.UnbondingId)
		s.Require().Equal(genesisState.UnbondingMappings[i].PositionId, m.PositionId)
	}
}

func (s *KeeperSuite) TestInitExportGenesis_SecondaryIndexesRebuilt() {
	owner := sdk.AccAddress([]byte("genesis_idx_owner___"))
	valAddr := sdk.ValAddress([]byte("genesis_idx_val_____"))
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	genesisState := &types.GenesisState{
		Params: types.DefaultParams(),
		Tiers: []types.Tier{
			{Id: 1, ExitDuration: time.Hour * 24, BonusApy: sdkmath.LegacyNewDecWithPrec(4, 2), MinLockAmount: sdkmath.NewInt(100)},
			{Id: 2, ExitDuration: time.Hour * 48, BonusApy: sdkmath.LegacyNewDecWithPrec(2, 2), MinLockAmount: sdkmath.NewInt(100)},
		},
		Positions: []types.Position{
			{
				Id: 1, Owner: owner.String(), TierId: 1, Amount: sdkmath.NewInt(1000),
				DelegatedShares: sdkmath.LegacyZeroDec(),
				CreatedAtHeight: 10, CreatedAtTime: now,
			},
			{
				Id: 2, Owner: owner.String(), TierId: 1, Amount: sdkmath.NewInt(2000),
				Validator: valAddr.String(), DelegatedShares: sdkmath.LegacyNewDec(2000),
				LastBonusAccrual: now,
				CreatedAtHeight:  11, CreatedAtTime: now,
			},
			{
				Id: 3, Owner: owner.String(), TierId: 2, Amount: sdkmath.NewInt(500),
				DelegatedShares: sdkmath.LegacyZeroDec(),
				CreatedAtHeight: 12, CreatedAtTime: now,
			},
		},
		NextPositionId: 4,
	}

	s.keeper.InitGenesis(s.ctx, genesisState)

	// Verify PositionsByOwner index.
	ids, err := s.keeper.GetPositionsIdsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().ElementsMatch([]uint64{1, 2, 3}, ids)

	// Verify PositionCountByTier.
	count1, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count1)

	count2, err := s.keeper.GetPositionCountForTier(s.ctx, 2)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count2)

	// Verify PositionsByValidator index.
	valIds, err := s.keeper.GetPositionsIdsByValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().ElementsMatch([]uint64{2}, valIds)
}

func (s *KeeperSuite) TestInitExportGenesis_SequenceContinuity() {
	owner := sdk.AccAddress([]byte("genesis_seq_owner___")).String()
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	genesisState := &types.GenesisState{
		Params: types.DefaultParams(),
		Tiers: []types.Tier{
			{Id: 1, ExitDuration: time.Hour * 24, BonusApy: sdkmath.LegacyNewDecWithPrec(4, 2), MinLockAmount: sdkmath.NewInt(100)},
		},
		Positions: []types.Position{
			{
				Id: 5, Owner: owner, TierId: 1, Amount: sdkmath.NewInt(1000),
				DelegatedShares: sdkmath.LegacyZeroDec(),
				CreatedAtHeight: 10, CreatedAtTime: now,
			},
		},
		NextPositionId: 10,
	}

	s.keeper.InitGenesis(s.ctx, genesisState)

	// Verify the sequence was set correctly.
	nextId, err := s.keeper.NextPositionId.Peek(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(uint64(10), nextId)
}

func (s *KeeperSuite) TestInitExportGenesis_DefaultRoundTrip() {
	defaultGenesis := types.DefaultGenesisState()
	s.keeper.InitGenesis(s.ctx, defaultGenesis)

	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().True(exported.Params.TargetBaseRewardsRate.IsZero())
	s.Require().Empty(exported.Tiers)
	s.Require().Empty(exported.Positions)
	s.Require().Equal(uint64(0), exported.NextPositionId)
	s.Require().Empty(exported.ValidatorRewardRatios)
	s.Require().Empty(exported.UnbondingMappings)
}
