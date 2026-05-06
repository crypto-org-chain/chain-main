package keeper_test

import (
	"fmt"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TestInitExportGenesis_FullRoundTrip exercises a full import/export cycle
// with a mix of delegated and undelegated positions, unbonding + redelegation
// mappings, and validator events.
func (s *KeeperSuite) TestInitExportGenesis_FullRoundTrip() {
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	owner := sdk.AccAddress([]byte("genesis_test_owner__")).String()
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	lockAmount := sdkmath.NewInt(1000)

	// Seed staking delegations for positions 1 and 2 so InitGenesis can resolve
	// their validator via stakingKeeper.GetDelegatorDelegations. Position 3 is
	// intentionally left undelegated.
	for _, id := range []uint64{1, 2} {
		delAddr := types.GetDelegatorAddress(id)
		s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
			sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
		_, err := s.app.StakingKeeper.Delegate(s.ctx, delAddr, lockAmount, stakingtypes.Unbonded, val, true)
		s.Require().NoError(err)
	}

	// Seed an active staking redelegation for position 1.
	dstValAddr, _ := s.createSecondValidator()
	del1, err := s.app.StakingKeeper.GetDelegation(s.ctx, types.GetDelegatorAddress(1), valAddr)
	s.Require().NoError(err)
	_, unbondingID1, err := s.app.StakingKeeper.BeginRedelegation(
		s.ctx, types.GetDelegatorAddress(1), valAddr, dstValAddr, del1.Shares,
	)
	s.Require().NoError(err)
	s.Require().NotZero(unbondingID1)

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

	// Two delegated positions on the same validator.
	pos1 := types.NewPosition(1, owner, 1, 100, 0, now, true, now)
	pos2 := types.NewPosition(2, owner, 2, 101, 0, now, true, now)

	// Undelegated, exit-triggered position.
	pos3 := types.NewPosition(3, owner, 1, 102, 0, time.Time{}, false, now)
	pos3.TriggerExit(now.Add(-time.Hour), time.Hour*24)

	genesisState := &types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier1, tier2},
		Positions:      []types.Position{pos1, pos2, pos3},
		NextPositionId: 4,
		RedelegationMappings: []types.RedelegationMapping{
			{UnbondingId: unbondingID1, PositionId: 1},
		},
		ValidatorEvents: []types.ValidatorEventEntry{
			{
				Validator: valAddr.String(),
				Sequence:  1,
				Event: types.ValidatorEvent{
					Height:         50,
					Timestamp:      now.Add(-time.Hour),
					EventType:      types.ValidatorEventType_VALIDATOR_EVENT_TYPE_SLASH,
					TokensPerShare: sdkmath.LegacyOneDec(),
					// pos2 is the only position with LastEventSeq=0 still on valAddr
					// (pos1 was fully redelegated above; pos3 is undelegated).
					ReferenceCount: 1,
				},
			},
		},
		ValidatorEventSeqs: []types.ValidatorEventSeqEntry{
			{Validator: valAddr.String(), CurrentSeq: 1},
		},
	}

	s.keeper.InitGenesis(s.ctx, genesisState)

	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().Equal(genesisState.Params.TargetBaseRewardsRate.String(), exported.Params.TargetBaseRewardsRate.String())
	s.Require().Len(exported.Tiers, 2)
	s.Require().Len(exported.Positions, 3)
	s.Require().Equal(uint64(4), exported.NextPositionId)
	s.Require().Len(exported.RedelegationMappings, 1)
	s.Require().Len(exported.ValidatorEvents, 1)
	s.Require().Len(exported.ValidatorEventSeqs, 1)

	// Per-position fields survive the round trip.
	byID := make(map[uint64]types.Position, len(exported.Positions))
	for _, p := range exported.Positions {
		byID[p.Id] = p
	}
	for _, orig := range []types.Position{pos1, pos2, pos3} {
		got, ok := byID[orig.Id]
		s.Require().True(ok, "position %d should be exported", orig.Id)
		s.Require().Equal(orig.Owner, got.Owner)
		s.Require().Equal(orig.TierId, got.TierId)
		s.Require().Equal(orig.CreatedAtHeight, got.CreatedAtHeight)
		s.Require().Equal(orig.CreatedAtTime.UTC(), got.CreatedAtTime.UTC())
		s.Require().Equal(orig.LastKnownBonded, got.LastKnownBonded)
		s.Require().Equal(orig.LastEventSeq, got.LastEventSeq)
		s.Require().Equal(orig.LastBonusAccrual.UTC(), got.LastBonusAccrual.UTC())
		s.Require().Equal(orig.ExitTriggeredAt.UTC(), got.ExitTriggeredAt.UTC())
		s.Require().Equal(orig.ExitUnlockAt.UTC(), got.ExitUnlockAt.UTC())
	}

	// Validator event preserved verbatim.
	s.Require().Equal(valAddr.String(), exported.ValidatorEvents[0].Validator)
	s.Require().Equal(uint64(1), exported.ValidatorEvents[0].Sequence)
	s.Require().Equal(uint64(1), exported.ValidatorEvents[0].Event.ReferenceCount)

	// Event sequence preserved.
	s.Require().Equal(valAddr.String(), exported.ValidatorEventSeqs[0].Validator)
	s.Require().Equal(uint64(1), exported.ValidatorEventSeqs[0].CurrentSeq)
}

// TestInitExportGenesis_SecondaryIndexesRebuilt verifies that InitGenesis
// rebuilds the derived indexes (PositionsByOwner, PositionCountByTier,
// PositionCountByValidator) purely from the position list — none of these are
// stored in genesis state.
func (s *KeeperSuite) TestInitExportGenesis_SecondaryIndexesRebuilt() {
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	owner := sdk.AccAddress([]byte("genesis_idx_owner___"))
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	lockAmount := sdkmath.NewInt(1000)

	// Only pos2 is delegated.
	delAddr := types.GetDelegatorAddress(2)
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
	_, err := s.app.StakingKeeper.Delegate(s.ctx, delAddr, lockAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	tier1 := types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24 * 365,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(100),
	}
	tier2 := types.Tier{
		Id:            2,
		ExitDuration:  time.Hour * 24 * 30,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(2, 2),
		MinLockAmount: sdkmath.NewInt(100),
	}

	// Undelegated with exit triggered, tier 1.
	pos1 := types.NewPosition(1, owner.String(), 1, 100, 0, time.Time{}, false, now)
	pos1.TriggerExit(now.Add(-time.Hour), time.Hour*24)

	// Delegated, tier 1.
	pos2 := types.NewPosition(2, owner.String(), 1, 101, 0, now, true, now)

	// Undelegated, no exit, tier 2.
	pos3 := types.NewPosition(3, owner.String(), 2, 102, 0, time.Time{}, false, now)

	genesisState := &types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier1, tier2},
		Positions:      []types.Position{pos1, pos2, pos3},
		NextPositionId: 4,
	}

	s.keeper.InitGenesis(s.ctx, genesisState)

	ids, err := s.keeper.GetPositionsIdsByOwner(s.ctx, owner)
	s.Require().NoError(err)
	s.Require().ElementsMatch([]uint64{1, 2, 3}, ids)

	count1, err := s.keeper.GetPositionCountForTier(s.ctx, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), count1)

	count2, err := s.keeper.GetPositionCountForTier(s.ctx, 2)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count2)

	// Only pos2 is delegated on this validator; the other two positions are
	// undelegated and must not affect the validator counter.
	valCount, err := s.keeper.GetPositionCountForValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount)
}

// TestInitExportGenesis_SequenceContinuity verifies that NextPositionId from
// genesis state is preserved, even when it's higher than the max position ID.
func (s *KeeperSuite) TestInitExportGenesis_SequenceContinuity() {
	owner := sdk.AccAddress([]byte("genesis_seq_owner___")).String()
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	tier := types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(100),
	}

	// Single undelegated, exit-triggered position with a non-contiguous id.
	pos5 := types.NewPosition(5, owner, 1, 10, 0, time.Time{}, false, now)
	pos5.TriggerExit(now.Add(-time.Hour), time.Hour*24)

	genesisState := &types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos5},
		NextPositionId: 10,
	}

	s.keeper.InitGenesis(s.ctx, genesisState)

	nextId, err := s.keeper.NextPositionId.Peek(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(uint64(10), nextId)
}

// TestInitExportGenesis_DefaultRoundTrip verifies that the default (empty)
// genesis imports and exports cleanly.
func (s *KeeperSuite) TestInitExportGenesis_DefaultRoundTrip() {
	defaultGenesis := types.DefaultGenesisState()
	s.keeper.InitGenesis(s.ctx, defaultGenesis)

	exported := s.keeper.ExportGenesis(s.ctx)
	s.Require().NotNil(exported)
	s.Require().True(exported.Params.TargetBaseRewardsRate.IsZero())
	s.Require().Empty(exported.Tiers)
	s.Require().Empty(exported.Positions)
	s.Require().Equal(uint64(0), exported.NextPositionId)
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

// TestInitGenesis_PanicsOnPhantomRedelegationMapping verifies that InitGenesis
// rejects any RedelegationMappings entry whose unbonding id has no matching
// active redelegation entry in the staking module.
func (s *KeeperSuite) TestInitGenesis_PanicsOnPhantomRedelegationMapping() {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	owner := sdk.AccAddress([]byte("genesis_phantom_own_")).String()
	tier := types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(100),
	}
	// Undelegated position
	pos := types.NewPosition(1, owner, 1, 100, 0, time.Time{}, false, now)

	genesisState := &types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos},
		NextPositionId: 2,
		RedelegationMappings: []types.RedelegationMapping{
			{UnbondingId: 99999, PositionId: 1},
		},
	}

	s.Require().PanicsWithError(
		"redelegation mapping (unbonding_id=99999, position_id=1) has no matching staking redelegation: no redelegation found",
		func() { s.keeper.InitGenesis(s.ctx, genesisState) },
	)
}

// TestInitGenesis_AcceptsValidRedelegationMapping verifies that InitGenesis
// accepts a RedelegationMappings entry whose unbondingId is backed by a real
// active staking redelegation entry, and that the mapping is loaded.
func (s *KeeperSuite) TestInitGenesis_AcceptsValidRedelegationMapping() {
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	owner := sdk.AccAddress([]byte("genesis_valid_owner_")).String()
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	lockAmount := sdkmath.NewInt(1000)
	delAddr := types.GetDelegatorAddress(1)
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
	_, err := s.app.StakingKeeper.Delegate(s.ctx, delAddr, lockAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	dstValAddr, _ := s.createSecondValidator()
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	redelegateShares := del.Shares.Quo(sdkmath.LegacyNewDec(2))
	_, unbondingID, err := s.app.StakingKeeper.BeginRedelegation(s.ctx, delAddr, valAddr, dstValAddr, redelegateShares)
	s.Require().NoError(err)
	s.Require().NotZero(unbondingID)

	tier := types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(100),
	}
	pos := types.NewPosition(1, owner, 1, 100, 0, now, true, now)

	genesisState := &types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos},
		NextPositionId: 2,
		RedelegationMappings: []types.RedelegationMapping{
			{UnbondingId: unbondingID, PositionId: 1},
		},
	}

	s.Require().NotPanics(func() { s.keeper.InitGenesis(s.ctx, genesisState) })

	gotPosID, err := s.keeper.RedelegationMappings.Get(s.ctx, unbondingID)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), gotPosID)
}

func (s *KeeperSuite) TestInitGenesis_PanicsOnRedelegationMappingDelegatorMismatch() {
	vals, bondDenom := s.getStakingData()
	val := vals[0]
	valAddr := sdk.MustValAddressFromBech32(val.GetOperator())

	owner := sdk.AccAddress([]byte("genesis_mismatch_own")).String()
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	s.ctx = s.ctx.WithBlockTime(now)

	// Real staking redelegation issued by GetDelegatorAddress(1) — i.e. owned
	// by position 1.
	lockAmount := sdkmath.NewInt(1000)
	delAddr := types.GetDelegatorAddress(1)
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, delAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, lockAmount))))
	_, err := s.app.StakingKeeper.Delegate(s.ctx, delAddr, lockAmount, stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	dstValAddr, _ := s.createSecondValidator()
	del, err := s.app.StakingKeeper.GetDelegation(s.ctx, delAddr, valAddr)
	s.Require().NoError(err)
	_, unbondingID, err := s.app.StakingKeeper.BeginRedelegation(s.ctx, delAddr, valAddr, dstValAddr, del.Shares.Quo(sdkmath.LegacyNewDec(2)))
	s.Require().NoError(err)
	s.Require().NotZero(unbondingID)

	tier := types.Tier{
		Id:            1,
		ExitDuration:  time.Hour * 24,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(100),
	}
	pos1 := types.NewPosition(1, owner, 1, 100, 0, now, true, now)
	pos2 := types.NewPosition(2, owner, 1, 100, 0, time.Time{}, false, now)

	// Mapping pairs the real unbondingId (owned by position 1) with position 2.
	genesisState := &types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos1, pos2},
		NextPositionId: 3,
		RedelegationMappings: []types.RedelegationMapping{
			{UnbondingId: unbondingID, PositionId: 2},
		},
	}

	s.Require().PanicsWithError(
		fmt.Sprintf(
			"redelegation mapping (unbonding_id=%d, position_id=2) delegator mismatch: staking has %q, expected %q",
			unbondingID, delAddr.String(), types.GetDelegatorAddress(2).String(),
		),
		func() { s.keeper.InitGenesis(s.ctx, genesisState) },
	)
}
