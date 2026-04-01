package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func genesisTier(id uint32) types.Tier {
	return types.Tier{
		Id:            id,
		ExitDuration:  time.Hour * 24 * 365,
		BonusApy:      sdkmath.LegacyNewDecWithPrec(4, 2),
		MinLockAmount: sdkmath.NewInt(1000),
	}
}

func genesisPosition(id uint64, tierId uint32) types.Position {
	return types.NewPosition(id, testOwner, tierId, sdkmath.NewInt(1000), 100, types.Delegation{
		Validator:           testValidator,
		Shares:              sdkmath.LegacyNewDec(1000),
		BaseRewardsPerShare: sdk.DecCoins{},
	}, time.Now())
}

func validFullGenesis() types.GenesisState {
	tier := genesisTier(1)
	pos := genesisPosition(1, 1)
	return types.GenesisState{
		Params:         types.DefaultParams(),
		Tiers:          []types.Tier{tier},
		Positions:      []types.Position{pos},
		NextPositionId: 2,
		ValidatorRewardRatios: []types.ValidatorRewardRatioEntry{
			{
				Validator:   testValidator,
				RewardRatio: types.ValidatorRewardRatio{},
			},
		},
		UnbondingDelegationMappings: []types.UnbondingMapping{
			{UnbondingId: 10, PositionId: 1},
		},
		RedelegationMappings: []types.UnbondingMapping{
			{UnbondingId: 11, PositionId: 1},
		},
	}
}

func TestValidateGenesis(t *testing.T) {
	t.Run("valid default genesis", func(t *testing.T) {
		genesis := types.DefaultGenesisState()
		require.NoError(t, types.ValidateGenesis(*genesis))
	})

	t.Run("valid custom genesis", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.NewParams(sdkmath.LegacyNewDecWithPrec(3, 2)),
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("invalid genesis - negative rate", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.NewParams(sdkmath.LegacyNewDec(-1)),
		}
		require.Error(t, types.ValidateGenesis(genesis))
	})

	t.Run("valid full genesis", func(t *testing.T) {
		genesis := validFullGenesis()
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("duplicate tier IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Tiers = append(genesis.Tiers, genesisTier(1))
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate tier ID")
	})

	t.Run("invalid tier fields", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Tiers[0].ExitDuration = 0 // invalid
		require.ErrorContains(t, types.ValidateGenesis(genesis), "invalid tier")
	})

	t.Run("duplicate position IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Positions = append(genesis.Positions, genesisPosition(1, 1))
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate position ID")
	})

	t.Run("position references unknown tier", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.Positions[0].TierId = 99
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown tier ID")
	})

	t.Run("NextPositionId too low", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.NextPositionId = 1 // must be > position ID 1
		require.ErrorContains(t, types.ValidateGenesis(genesis), "next_position_id")
	})

	t.Run("invalid validator address in reward ratio", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorRewardRatios[0].Validator = "invalid"
		require.ErrorContains(t, types.ValidateGenesis(genesis), "invalid validator address")
	})

	t.Run("duplicate validator in reward ratios", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.ValidatorRewardRatios = append(genesis.ValidatorRewardRatios, genesis.ValidatorRewardRatios[0])
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate validator")
	})

	t.Run("unbonding mapping references unknown position", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.UnbondingDelegationMappings[0].PositionId = 999
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown position ID")
	})

	t.Run("duplicate unbonding IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.UnbondingDelegationMappings = append(genesis.UnbondingDelegationMappings, types.UnbondingMapping{UnbondingId: 10, PositionId: 1})
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate unbonding ID")
	})

	t.Run("redelegation mapping references unknown position", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings[0].PositionId = 999
		require.ErrorContains(t, types.ValidateGenesis(genesis), "unknown position ID")
	})

	t.Run("duplicate redelegation IDs", func(t *testing.T) {
		genesis := validFullGenesis()
		genesis.RedelegationMappings = append(genesis.RedelegationMappings, types.UnbondingMapping{UnbondingId: 11, PositionId: 1})
		require.ErrorContains(t, types.ValidateGenesis(genesis), "duplicate redelegation ID")
	})
}
