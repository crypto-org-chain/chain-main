package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Test-only wrappers for black-box tests (package keeper_test) that need access
// to unexported keeper APIs. These are compiled only when running tests.

func (k Keeper) SetPosition(ctx context.Context, pos types.Position) error {
	return k.setPosition(ctx, pos)
}

func (k Keeper) GetPosition(ctx context.Context, id uint64) (types.Position, error) {
	return k.getPosition(ctx, id)
}

func (k Keeper) DeletePosition(ctx context.Context, pos types.Position) error {
	return k.deletePosition(ctx, pos)
}

func (k Keeper) GetTier(ctx context.Context, id uint32) (types.Tier, error) {
	return k.getTier(ctx, id)
}

func (k Keeper) SetTier(ctx context.Context, tier types.Tier) error {
	return k.setTier(ctx, tier)
}

func (k Keeper) HasTier(ctx context.Context, id uint32) (bool, error) {
	return k.hasTier(ctx, id)
}

func (k Keeper) DeleteTier(ctx context.Context, tierId uint32) error {
	return k.deleteTier(ctx, tierId)
}

func (k Keeper) GetPositionsByOwner(ctx context.Context, owner sdk.AccAddress) ([]types.Position, error) {
	return k.getPositionsByOwner(ctx, owner)
}

func (k Keeper) GetPositionsByValidator(ctx context.Context, valAddr sdk.ValAddress) ([]types.Position, error) {
	return k.getPositionsByValidator(ctx, valAddr)
}

func (k Keeper) GetPositionsIdsByOwner(ctx context.Context, owner sdk.AccAddress) ([]uint64, error) {
	return k.getPositionsIdsByOwner(ctx, owner)
}

func (k Keeper) GetPositionsIdsByValidator(ctx context.Context, valAddr sdk.ValAddress) ([]uint64, error) {
	return k.getPositionsIdsByValidator(ctx, valAddr)
}

func (k Keeper) GetPositionCountForTier(ctx context.Context, tierId uint32) (uint64, error) {
	return k.getPositionCountForTier(ctx, tierId)
}

func (k Keeper) HasActivePositionsForTier(ctx context.Context, tierId uint32) (bool, error) {
	return k.hasActivePositionsForTier(ctx, tierId)
}

func (k Keeper) TransferDelegation(ctx context.Context, delegatorAddr, validatorAddr string, amount math.Int) (math.LegacyDec, error) {
	return k.transferDelegation(ctx, delegatorAddr, validatorAddr, amount)
}

func (k Keeper) GetVotingPowerForAddress(ctx context.Context, voter sdk.AccAddress) (math.LegacyDec, error) {
	return k.getVotingPowerForAddress(ctx, voter)
}

func (k Keeper) TotalDelegatedVotingPower(ctx context.Context) (math.LegacyDec, error) {
	return k.totalDelegatedVotingPower(ctx)
}

func (k Keeper) ClaimBonusRewards(ctx context.Context, pos *types.Position, tier types.Tier, forceAccrue bool) (sdk.Coins, error) {
	return k.claimBonusRewards(ctx, pos, tier, forceAccrue)
}

func (k Keeper) SettleRewardsForPositions(ctx context.Context, valAddr sdk.ValAddress, positions []types.Position, forceAccrue bool) error {
	return k.settleRewardsForPositions(ctx, valAddr, positions, forceAccrue)
}

func (k Keeper) GetAuthority() string {
	return k.getAuthority()
}

func (k Keeper) LockFunds(ctx context.Context, owner string, amount math.Int) error {
	return k.lockFunds(ctx, owner, amount)
}

func (k Keeper) Delegate(ctx context.Context, valAddr sdk.ValAddress, amount math.Int) (math.LegacyDec, error) {
	return k.delegate(ctx, valAddr, amount)
}

func (k Keeper) GetValidatorRewardRatio(ctx context.Context, valAddr sdk.ValAddress) (sdk.DecCoins, error) {
	return k.getValidatorRewardRatio(ctx, valAddr)
}

func (k Keeper) CreatePosition(
	ctx context.Context,
	owner string,
	tier types.Tier,
	amount math.Int,
	delegation *types.Delegation,
	triggerExitImmediately bool,
) (types.Position, error) {
	return k.createPosition(ctx, owner, tier, amount, delegation, triggerExitImmediately)
}

func (k Keeper) GetPositionsByIds(ctx context.Context, ids []uint64) ([]types.Position, error) {
	return k.getPositionsByIds(ctx, ids)
}

func (k Keeper) StillUnbonding(ctx context.Context, positionId uint64) (bool, error) {
	return k.stillUnbonding(ctx, positionId)
}
