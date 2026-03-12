package types_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func TestValidateGenesis(t *testing.T) {
	t.Run("valid default genesis", func(t *testing.T) {
		genesis := types.DefaultGenesisState()
		require.NoError(t, types.ValidateGenesis(*genesis))
	})

	t.Run("valid custom genesis", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.NewParams(sdkmath.LegacyNewDecWithPrec(3, 2), []types.TierDefinition{}, []string{}),
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})

	t.Run("invalid genesis - negative rate", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.NewParams(sdkmath.LegacyNewDec(-1), []types.TierDefinition{}, []string{}),
		}
		require.Error(t, types.ValidateGenesis(genesis))
	})

	// MED-1: Validate positions in genesis.
	t.Run("invalid genesis - duplicate position IDs", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			Positions: []types.TierPosition{
				{PositionId: 1, Owner: "cosmos1abc", AmountLocked: sdkmath.NewInt(100)},
				{PositionId: 1, Owner: "cosmos1def", AmountLocked: sdkmath.NewInt(200)},
			},
			NextPositionId: 2,
		}
		err := types.ValidateGenesis(genesis)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate position ID")
	})

	t.Run("invalid genesis - empty owner", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			Positions: []types.TierPosition{
				{PositionId: 1, Owner: "", AmountLocked: sdkmath.NewInt(100)},
			},
			NextPositionId: 2,
		}
		err := types.ValidateGenesis(genesis)
		require.Error(t, err)
		require.Contains(t, err.Error(), "owner cannot be empty")
	})

	t.Run("invalid genesis - negative amount", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			Positions: []types.TierPosition{
				{PositionId: 1, Owner: "cosmos1abc", AmountLocked: sdkmath.NewInt(-100)},
			},
			NextPositionId: 2,
		}
		err := types.ValidateGenesis(genesis)
		require.Error(t, err)
		require.Contains(t, err.Error(), "amount_locked cannot be negative")
	})

	t.Run("invalid genesis - next_position_id too low", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			Positions: []types.TierPosition{
				{PositionId: 5, Owner: "cosmos1abc", AmountLocked: sdkmath.NewInt(100)},
			},
			NextPositionId: 3, // must be > 5
		}
		err := types.ValidateGenesis(genesis)
		require.Error(t, err)
		require.Contains(t, err.Error(), "next_position_id")
	})

	t.Run("valid genesis - with positions", func(t *testing.T) {
		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			Positions: []types.TierPosition{
				{PositionId: 1, Owner: "cosmos1abc", AmountLocked: sdkmath.NewInt(100)},
				{PositionId: 2, Owner: "cosmos1def", AmountLocked: sdkmath.NewInt(200)},
			},
			NextPositionId: 3,
		}
		require.NoError(t, types.ValidateGenesis(genesis))
	})
}

func TestParams_ValidateTiers_MaxBonusAPY(t *testing.T) {
	t.Run("valid bonus APY at max", func(t *testing.T) {
		tiers := []types.TierDefinition{{
			TierId:                        1,
			ExitCommitmentDuration:        time.Hour * 24 * 365,
			ExitCommitmentDurationInYears: 1,
			BonusApy:                      sdkmath.LegacyNewDec(10), // exactly at max (1000%)
			MinLockAmount:                 sdkmath.NewInt(1000),
		}}
		params := types.NewParams(sdkmath.LegacyZeroDec(), tiers, []string{})
		require.NoError(t, params.Validate())
	})

	t.Run("invalid bonus APY exceeds max", func(t *testing.T) {
		tiers := []types.TierDefinition{{
			TierId:                        1,
			ExitCommitmentDuration:        time.Hour * 24 * 365,
			ExitCommitmentDurationInYears: 1,
			BonusApy:                      sdkmath.LegacyNewDec(11), // exceeds 1000% cap
			MinLockAmount:                 sdkmath.NewInt(1000),
		}}
		params := types.NewParams(sdkmath.LegacyZeroDec(), tiers, []string{})
		err := params.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds maximum")
	})
}
