package keeper_test

import (
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

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

	// Delegated position (tier 1). Amount must be zero.
	pos1 := types.Position{
		Id:               1,
		Owner:            owner,
		TierId:           1,
		Amount:           sdkmath.ZeroInt(),
		Validator:        valAddr.String(),
		DelegatedShares:  sdkmath.LegacyNewDec(5000),
		LastBonusAccrual: now,
		CreatedAtHeight:  100,
		CreatedAtTime:    now,
		LastEventSeq:     0,
		LastKnownBonded:  true,
	}

	// Delegated position. Amount must be zero.
	pos2 := types.Position{
		Id:               2,
		Owner:            owner,
		TierId:           2,
		Amount:           sdkmath.ZeroInt(),
		Validator:        valAddr.String(),
		DelegatedShares:  sdkmath.LegacyNewDec(3000),
		LastBonusAccrual: now,
		CreatedAtHeight:  101,
		CreatedAtTime:    now,
		LastEventSeq:     0,
		LastKnownBonded:  true,
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
		LastKnownBonded: false,
	}

	genesisState := &types.GenesisState{
		Params:         types.NewParams(sdkmath.LegacyNewDecWithPrec(5, 2)),
		Tiers:          []types.Tier{tier1, tier2},
		Positions:      []types.Position{pos1, pos2, pos3},
		NextPositionId: 4,
		UnbondingDelegationMappings: []types.UnbondingMapping{
			{UnbondingId: 42, PositionId: 2},
			{UnbondingId: 43, PositionId: 3},
		},
		RedelegationMappings: []types.UnbondingMapping{
			{UnbondingId: 44, PositionId: 2},
			{UnbondingId: 45, PositionId: 3},
		},
		ValidatorEvents: []types.ValidatorEventEntry{
			{
				Validator: valAddr.String(),
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         100,
					Timestamp:      now.Add(-10 * time.Second),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 2,
				},
			},
			{
				Validator: valAddr.String(),
				Sequence:  2,
				Event: types.ValidatorEvent{
					Height:         101,
					Timestamp:      now.Add(-5 * time.Second),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_UNBOND,
					TokensPerShare: sdkmath.LegacyOneDec(),
					ReferenceCount: 2,
				},
			},
		},
		ValidatorEventSeqs: []types.ValidatorEventSeqEntry{
			{Validator: valAddr.String(), CurrentSeq: 2},
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
		s.Require().Equal(orig.LastBonusAccrual, pos.LastBonusAccrual)
		s.Require().Equal(orig.CreatedAtHeight, pos.CreatedAtHeight)
		s.Require().Equal(orig.CreatedAtTime.UTC(), pos.CreatedAtTime.UTC())
		s.Require().Equal(orig.ExitTriggeredAt.UTC(), pos.ExitTriggeredAt.UTC())
		s.Require().Equal(orig.ExitUnlockAt.UTC(), pos.ExitUnlockAt.UTC())
		s.Require().Equal(orig.LastEventSeq, pos.LastEventSeq)
		s.Require().Equal(orig.LastKnownBonded, pos.LastKnownBonded)
	}

	// Sequence.
	s.Require().Equal(genesisState.NextPositionId, exported.NextPositionId)

	// Unbonding delegation mappings.
	s.Require().Len(exported.UnbondingDelegationMappings, 2)
	for i, m := range exported.UnbondingDelegationMappings {
		s.Require().Equal(genesisState.UnbondingDelegationMappings[i].UnbondingId, m.UnbondingId)
		s.Require().Equal(genesisState.UnbondingDelegationMappings[i].PositionId, m.PositionId)
	}
	// Redelegation mappings.
	s.Require().Len(exported.RedelegationMappings, 2)
	for i, m := range exported.RedelegationMappings {
		s.Require().Equal(genesisState.RedelegationMappings[i].UnbondingId, m.UnbondingId)
		s.Require().Equal(genesisState.RedelegationMappings[i].PositionId, m.PositionId)
	}

	// Validator events.
	s.Require().Len(exported.ValidatorEvents, 2)
	for i, e := range exported.ValidatorEvents {
		orig := genesisState.ValidatorEvents[i]
		s.Require().Equal(orig.Validator, e.Validator, "event validator mismatch at %d", i)
		s.Require().Equal(orig.Sequence, e.Sequence, "event sequence mismatch at %d", i)
		s.Require().Equal(orig.Event.EventType, e.Event.EventType, "event type mismatch at %d", i)
		s.Require().True(orig.Event.TokensPerShare.Equal(e.Event.TokensPerShare), "event tokens_per_share mismatch at %d", i)
		s.Require().Equal(orig.Event.ReferenceCount, e.Event.ReferenceCount, "event reference count mismatch at %d", i)
		s.Require().Equal(orig.Event.Height, e.Event.Height, "event height mismatch at %d", i)
		s.Require().Equal(orig.Event.Timestamp.UTC(), e.Event.Timestamp.UTC(), "event timestamp mismatch at %d", i)
	}

	// Validator event current sequences.
	s.Require().Len(exported.ValidatorEventSeqs, 1)
	s.Require().Equal(genesisState.ValidatorEventSeqs[0].Validator, exported.ValidatorEventSeqs[0].Validator)
	s.Require().Equal(genesisState.ValidatorEventSeqs[0].CurrentSeq, exported.ValidatorEventSeqs[0].CurrentSeq)

	// Validator position counts are rebuilt by setPosition during InitGenesis,
	// not stored in genesis. Verify they were rebuilt correctly.
	count, err := s.keeper.PositionCountByValidator.Get(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count, "position count should be rebuilt from positions")
}

func (s *KeeperSuite) TestInitExportGenesis_SecondaryIndexesRebuilt() {
	owner := sdk.AccAddress([]byte("genesis_idx_owner___"))
	valAddr := sdk.ValAddress([]byte("genesis_idx_val_____"))
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	exitTime := now.Add(-time.Hour)

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
				ExitTriggeredAt: exitTime, ExitUnlockAt: exitTime.Add(time.Hour * 24),
				CreatedAtHeight: 10, CreatedAtTime: now,
			},
			{
				Id: 2, Owner: owner.String(), TierId: 1, Amount: sdkmath.ZeroInt(),
				Validator: valAddr.String(), DelegatedShares: sdkmath.LegacyNewDec(2000),
				LastBonusAccrual: now,
				CreatedAtHeight:  11, CreatedAtTime: now, LastEventSeq: 0, LastKnownBonded: true,
			},
			// simulate a redelegation-slashed to zero position here. No delegation here and amount is zero
			{
				Id: 3, Owner: owner.String(), TierId: 2, Amount: sdkmath.ZeroInt(),
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
	exitTime := now.Add(-time.Hour)

	genesisState := &types.GenesisState{
		Params: types.DefaultParams(),
		Tiers: []types.Tier{
			{Id: 1, ExitDuration: time.Hour * 24, BonusApy: sdkmath.LegacyNewDecWithPrec(4, 2), MinLockAmount: sdkmath.NewInt(100)},
		},
		Positions: []types.Position{
			{
				Id: 5, Owner: owner, TierId: 1, Amount: sdkmath.NewInt(1000),
				DelegatedShares: sdkmath.LegacyZeroDec(),
				ExitTriggeredAt: exitTime, ExitUnlockAt: exitTime.Add(time.Hour * 24),
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
	s.Require().Empty(exported.UnbondingDelegationMappings)
	s.Require().Empty(exported.RedelegationMappings)
}

func (s *KeeperSuite) TestInitGenesis_MaterializesModuleAccounts() {
	tierModuleAddr := s.app.AccountKeeper.GetModuleAddress(types.ModuleName)
	rewardsPoolAddr := s.app.AccountKeeper.GetModuleAddress(types.RewardsPoolName)

	s.keeper.InitGenesis(s.ctx, types.DefaultGenesisState())

	for _, addr := range []sdk.AccAddress{tierModuleAddr, rewardsPoolAddr} {
		acc := s.app.AccountKeeper.GetAccount(s.ctx, addr)
		s.Require().NotNil(acc, "module account should exist after InitGenesis")
		_, ok := acc.(sdk.ModuleAccountI)
		s.Require().True(ok, "account at %s should be a module account", addr.String())
	}
}
