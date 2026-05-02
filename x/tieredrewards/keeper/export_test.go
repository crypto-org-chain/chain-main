package keeper

import (
	"context"
	"time"

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

func (k Keeper) HasTier(ctx context.Context, id uint32) (bool, error) {
	return k.hasTier(ctx, id)
}

func (k Keeper) DeleteTier(ctx context.Context, tierId uint32) error {
	return k.deleteTier(ctx, tierId)
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

func (k Keeper) HasPositionsForTier(ctx context.Context, tierId uint32) (bool, error) {
	return k.hasPositionsForTier(ctx, tierId)
}

func (k Keeper) IncreasePositionCountForTier(ctx context.Context, tierId uint32) error {
	return k.increasePositionCountForTier(ctx, tierId)
}

func (k Keeper) DecreasePositionCountForTier(ctx context.Context, tierId uint32) error {
	return k.decreasePositionCountForTier(ctx, tierId)
}

func (k Keeper) GetPositionCountForValidator(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	return k.getPositionCountForValidator(ctx, valAddr)
}

func (k Keeper) IncreasePositionCountForValidator(ctx context.Context, valAddr sdk.ValAddress) error {
	return k.increasePositionCountForValidator(ctx, valAddr)
}

func (k Keeper) DecreasePositionCountForValidator(ctx context.Context, valAddr sdk.ValAddress) error {
	return k.decreasePositionCountForValidator(ctx, valAddr)
}

func (k Keeper) TransferDelegationFromPosition(ctx context.Context, pos types.Position, valAddr sdk.ValAddress, amount math.Int) (math.LegacyDec, math.LegacyDec, math.Int, error) {
	return k.transferDelegationFromPosition(ctx, pos, valAddr, amount)
}

func (k Keeper) TransferDelegationToPosition(ctx context.Context, owner string, posDelAddr sdk.AccAddress, validatorAddr string, amount math.Int) (math.LegacyDec, error) {
	return k.transferDelegationToPosition(ctx, owner, posDelAddr, validatorAddr, amount)
}

func (k Keeper) GetVotingPowerByOwner(ctx context.Context, owner sdk.AccAddress) (math.LegacyDec, error) {
	return k.getVotingPowerByOwner(ctx, owner)
}

func (k Keeper) TotalDelegatedVotingPower(ctx context.Context) (math.LegacyDec, error) {
	return k.totalDelegatedVotingPower(ctx)
}

func (k Keeper) GetAuthority() string {
	return k.getAuthority()
}

func (k Keeper) LockFunds(ctx context.Context, ownerAddr, delAddr sdk.AccAddress, amount math.Int) error {
	return k.lockFunds(ctx, ownerAddr, delAddr, amount)
}

func (k Keeper) CreatePosition(
	ctx context.Context,
	owner string,
	tier types.Tier,
	amount math.Int,
	delegation types.Delegation,
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

func (k Keeper) StillRedelegating(ctx context.Context, positionId uint64) (bool, error) {
	return k.stillRedelegating(ctx, positionId)
}

func (k Keeper) ComputeSegmentBonus(pos *types.Position, tier types.Tier, segmentStart, segmentEnd time.Time, tokensPerShare math.LegacyDec) math.Int {
	return k.computeSegmentBonus(pos, tier, segmentStart, segmentEnd, tokensPerShare)
}

func (k Keeper) GetTokensPerShare(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error) {
	return k.getTokensPerShare(ctx, valAddr)
}

func (k Keeper) ClaimRewardsAndUpdateTierPositions(ctx context.Context, tierId uint32) error {
	return k.claimRewardsAndUpdateTierPositions(ctx, tierId)
}

func (k Keeper) ClaimRewards(ctx context.Context, pos types.Position) (types.Position, sdk.Coins, sdk.Coins, error) {
	return k.claimRewards(ctx, pos)
}

func (k Keeper) ClaimRewardsAndUpdatesPositions(ctx context.Context, owner string, positions []types.Position) (sdk.Coins, sdk.Coins, error) {
	return k.claimRewardsAndUpdatesPositions(ctx, owner, positions)
}

func (k Keeper) PositionTokenValue(ctx context.Context, pos types.Position) (math.Int, error) {
	return k.positionTokenValue(ctx, pos)
}

func (k Keeper) AppendValidatorEvent(ctx context.Context, valAddr sdk.ValAddress, event types.ValidatorEvent) (uint64, error) {
	return k.appendValidatorEvent(ctx, valAddr, event)
}

func (k Keeper) GetValidatorEventsSince(ctx context.Context, valAddr sdk.ValAddress, startSeq uint64) ([]EventEntry, error) {
	return k.getValidatorEventsSince(ctx, valAddr, startSeq)
}

func (k Keeper) GetValidatorEventLatestSeq(ctx context.Context, valAddr sdk.ValAddress) (uint64, error) {
	return k.getValidatorEventLatestSeq(ctx, valAddr)
}

func (k Keeper) DecrementEventRefCount(ctx context.Context, valAddr sdk.ValAddress, seq uint64) error {
	return k.decrementEventRefCount(ctx, valAddr, seq)
}

func (k Keeper) DeleteValidatorEventSeq(ctx context.Context, valAddr sdk.ValAddress) error {
	return k.deleteValidatorEventSeq(ctx, valAddr)
}

func (k Keeper) HasValidatorEvents(ctx context.Context, valAddr sdk.ValAddress) (bool, error) {
	return k.hasValidatorEvents(ctx, valAddr)
}

func (k Keeper) ProcessEventsAndClaimBonus(ctx context.Context, pos *types.Position, valAddr sdk.ValAddress) (sdk.Coins, error) {
	return k.processEventsAndClaimBonus(ctx, pos, valAddr)
}

func (k Keeper) ClaimBaseRewards(ctx context.Context, pos types.Position, valAddr sdk.ValAddress) (sdk.Coins, error) {
	return k.claimBaseRewards(ctx, pos, valAddr)
}
