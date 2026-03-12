package keeper

import (
	"context"
	"fmt"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// CreatePosition creates a new tier position and returns it.
func (k Keeper) CreatePosition(ctx context.Context, owner string, tierId uint32, amountLocked math.Int) (types.TierPosition, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	positionId, err := k.NextPositionID.Next(ctx)
	if err != nil {
		return types.TierPosition{}, err
	}

	position := types.TierPosition{
		PositionId:      positionId,
		Owner:           owner,
		TierId:          tierId,
		AmountLocked:    amountLocked,
		CreatedAtHeight: sdkCtx.BlockHeight(),
		CreatedAtTime:   sdkCtx.BlockTime(),
		DelegatedShares: math.LegacyZeroDec(),
	}

	if err := k.Positions.Set(ctx, positionId, position); err != nil {
		return types.TierPosition{}, err
	}

	return position, nil
}

// GetPosition returns a position by ID.
func (k Keeper) GetPosition(ctx context.Context, positionId uint64) (types.TierPosition, error) {
	return k.Positions.Get(ctx, positionId)
}

// SetPosition updates a position.
func (k Keeper) SetPosition(ctx context.Context, position types.TierPosition) error {
	return k.Positions.Set(ctx, position.PositionId, position)
}

// DeletePosition removes a position.
func (k Keeper) DeletePosition(ctx context.Context, positionId uint64) error {
	return k.Positions.Remove(ctx, positionId)
}

// GetPositionsByOwner returns all positions owned by the given address using the owner index.
func (k Keeper) GetPositionsByOwner(ctx context.Context, owner string) ([]types.TierPosition, error) {
	var positions []types.TierPosition
	iter, err := k.Positions.Indexes.Owner.MatchExact(ctx, owner)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		pk, err := iter.PrimaryKey()
		if err != nil {
			return nil, err
		}
		pos, err := k.Positions.Get(ctx, pk)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, nil
}

// GetAllPositions returns all positions.
func (k Keeper) GetAllPositions(ctx context.Context) ([]types.TierPosition, error) {
	var positions []types.TierPosition
	iter, err := k.Positions.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		pos, err := iter.Value()
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, nil
}

// IsPositionExiting returns true if the position has triggered exit.
// Checks both Go zero-value and protobuf zero-value (Unix epoch) for timestamps.
func IsPositionExiting(pos types.TierPosition) bool {
	return !pos.ExitTriggeredAt.IsZero() && !pos.ExitTriggeredAt.Equal(time.Unix(0, 0))
}

// IsPositionDelegated returns true if the position is actively delegated to a validator.
// A position that is unbonding is NOT considered delegated (no rewards accrue during unbonding).
func IsPositionDelegated(pos types.TierPosition) bool {
	return pos.Validator != "" && !pos.IsUnbonding
}

// CalculateBonus computes the accrued bonus for a position.
// Returns the bonus amount (in bond denom units).
func (k Keeper) CalculateBonus(ctx context.Context, pos types.TierPosition, tier types.TierDefinition) (math.Int, error) {
	if !IsPositionDelegated(pos) {
		return math.ZeroInt(), nil
	}
	if pos.LastBonusAccrual.IsZero() || pos.LastBonusAccrual.Equal(time.Unix(0, 0)) {
		return math.ZeroInt(), nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	// Determine accrual end time
	accrualEnd := blockTime
	if IsPositionExiting(pos) && blockTime.After(pos.ExitUnlockTime) {
		accrualEnd = pos.ExitUnlockTime
	}

	// Duration in seconds using integer arithmetic (HIGH-5: fix float64)
	durationSeconds := int64(accrualEnd.Sub(pos.LastBonusAccrual) / time.Second)
	if durationSeconds <= 0 {
		return math.ZeroInt(), nil
	}

	// duration_years = seconds / SecondsPerYear
	durationYears := math.LegacyNewDec(durationSeconds).Quo(math.LegacyNewDec(types.SecondsPerYear))

	// accrued_bonus = AmountLocked * BonusApy * duration_years
	bonus := math.LegacyNewDecFromInt(pos.AmountLocked).
		Mul(tier.BonusApy).
		Mul(durationYears).
		TruncateInt()

	return bonus, nil
}

// GetVotingPowerForAddress returns the governance voting power from tier positions
// for an address. Uses the same shares-to-tokens formula as the governance tally
// (DelegatedShares * BondedTokens / DelegatorShares) and only counts positions
// on bonded validators, matching the tally's bonded-only filter.
func (k Keeper) GetVotingPowerForAddress(ctx context.Context, voterAddr string) (math.LegacyDec, error) {
	positions, err := k.GetPositionsByOwner(ctx, voterAddr)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	total := math.LegacyZeroDec()
	for _, pos := range positions {
		if !IsPositionDelegated(pos) || pos.DelegatedShares.IsNil() || !pos.DelegatedShares.IsPositive() {
			continue
		}
		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			continue
		}
		val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			continue // validator not found — skip
		}
		if !val.IsBonded() || val.GetDelegatorShares().IsZero() {
			continue // only bonded validators with non-zero shares, matching gov tally
		}
		tokenPower := pos.DelegatedShares.MulInt(val.GetBondedTokens()).Quo(val.GetDelegatorShares())
		total = total.Add(tokenPower)
	}
	return total, nil
}

// TransferDelegation transfers delegation shares from one delegator to another
// for the same validator, without unbonding.
// This properly integrates with the distribution module by settling rewards
// before share modification and reinitializing afterward (HIGH-1).
func (k Keeper) TransferDelegation(
	ctx context.Context,
	fromAddr sdk.AccAddress,
	toAddr sdk.AccAddress,
	valAddr sdk.ValAddress,
	shares math.LegacyDec,
) error {
	// Step 1: Settle existing rewards for source.
	if _, err := k.distributionKeeper.WithdrawDelegationRewards(ctx, fromAddr, valAddr); err != nil {
		// If source has no rewards yet, this may error; ignore gracefully.
		k.Logger(ctx).Debug("withdraw delegation rewards for source before transfer", "err", err)
	}

	// Step 2: Settle existing rewards for destination (if delegation exists).
	if _, err := k.stakingKeeper.GetDelegation(ctx, toAddr, valAddr); err == nil {
		if _, err := k.distributionKeeper.WithdrawDelegationRewards(ctx, toAddr, valAddr); err != nil {
			k.Logger(ctx).Debug("withdraw delegation rewards for dest before transfer", "err", err)
		}
	}

	// Step 3: Get the source delegation
	srcDelegation, err := k.stakingKeeper.GetDelegation(ctx, fromAddr, valAddr)
	if err != nil {
		return err
	}

	// Ensure enough shares
	if srcDelegation.GetShares().LT(shares) {
		return fmt.Errorf("insufficient delegation shares: have %s, want %s", srcDelegation.GetShares(), shares)
	}

	// Reduce source delegation shares
	newSrcShares := srcDelegation.GetShares().Sub(shares)

	if newSrcShares.IsZero() {
		// Remove the delegation entirely
		if err := k.stakingKeeper.RemoveDelegation(ctx, srcDelegation); err != nil {
			return err
		}
	} else {
		// Update with reduced shares
		srcDelegation.Shares = newSrcShares
		if err := k.stakingKeeper.SetDelegation(ctx, srcDelegation); err != nil {
			return err
		}
	}

	// Add to destination delegation
	dstDelegation, err := k.stakingKeeper.GetDelegation(ctx, toAddr, valAddr)
	if err != nil {
		// Delegation doesn't exist, create it
		dstDelegation = stakingtypes.NewDelegation(toAddr.String(), valAddr.String(), shares)
	} else {
		dstDelegation.Shares = dstDelegation.Shares.Add(shares)
	}

	if err := k.stakingKeeper.SetDelegation(ctx, dstDelegation); err != nil {
		return err
	}

	// Step 4: Reinitialize distribution tracking if available.
	if dk, ok := k.distributionKeeper.(types.DistributionKeeperWithInit); ok {
		if !newSrcShares.IsZero() {
			if err := dk.InitializeDelegation(ctx, valAddr, fromAddr); err != nil {
				return fmt.Errorf("failed to reinitialize source delegation: %w", err)
			}
		}
		if err := dk.InitializeDelegation(ctx, valAddr, toAddr); err != nil {
			return fmt.Errorf("failed to reinitialize dest delegation: %w", err)
		}
	}

	return nil
}
