package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

// reconcileAmountFromShares converts delegation shares to the actual withdrawable
// token amount under the validator's current exchange rate.
func (ms msgServer) reconcileAmountFromShares(ctx context.Context, valAddr sdk.ValAddress, shares sdkmath.LegacyDec) (sdkmath.Int, error) {
	val, err := ms.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return sdkmath.Int{}, err
	}
	return val.TokensFromShares(shares).TruncateInt(), nil
}

// applyDelegationToPosition updates a position's delegation fields and reconciles
// the stored amount from the validator's current share exchange rate.
func (ms msgServer) applyDelegationToPosition(ctx context.Context, pos *types.Position, delegation types.Delegation) error {
	valAddr, err := sdk.ValAddressFromBech32(delegation.Validator)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pos.WithDelegation(delegation, sdkCtx.BlockTime())

	reconciledAmount, err := ms.reconcileAmountFromShares(ctx, valAddr, delegation.Shares)
	if err != nil {
		return err
	}
	pos.UpdateAmount(reconciledAmount)

	return nil
}

func (ms msgServer) LockTier(ctx context.Context, msg *types.MsgLockTier) (*types.MsgLockTierResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	tier, err := ms.getTier(ctx, msg.Id)
	if err != nil {
		return nil, err
	}

	if err := ms.validateNewPosition(tier, msg.Amount); err != nil {
		return nil, err
	}

	if err := ms.lockFunds(ctx, msg.Owner, msg.Amount); err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}

	currentRatio, err := ms.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	shares, err := ms.delegate(ctx, valAddr, msg.Amount)
	if err != nil {
		return nil, err
	}

	delegation := types.Delegation{
		Validator:           msg.ValidatorAddress,
		Shares:              shares,
		BaseRewardsPerShare: currentRatio,
	}

	positionAmount, err := ms.reconcileAmountFromShares(ctx, valAddr, shares)
	if err != nil {
		return nil, err
	}

	pos, err := ms.createPosition(ctx, msg.Owner, tier, positionAmount, delegation, msg.TriggerExitImmediately)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionCreated{
		Position: pos,
	}); err != nil {
		return nil, err
	}

	return &types.MsgLockTierResponse{PositionId: pos.Id}, nil
}

func (ms msgServer) CommitDelegationToTier(ctx context.Context, msg *types.MsgCommitDelegationToTier) (*types.MsgCommitDelegationToTierResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	tier, err := ms.getTier(ctx, msg.Id)
	if err != nil {
		return nil, err
	}

	if err := ms.validateNewPosition(tier, msg.Amount); err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}

	currentRatio, err := ms.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	shares, err := ms.transferDelegation(ctx, msg.DelegatorAddress, msg.ValidatorAddress, msg.Amount)
	if err != nil {
		return nil, err
	}

	delegation := types.Delegation{
		Validator:           msg.ValidatorAddress,
		Shares:              shares,
		BaseRewardsPerShare: currentRatio,
	}

	positionAmount, err := ms.reconcileAmountFromShares(ctx, valAddr, shares)
	if err != nil {
		return nil, err
	}

	pos, err := ms.createPosition(ctx, msg.DelegatorAddress, tier, positionAmount, delegation, msg.TriggerExitImmediately)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventDelegationCommitted{
		Position: pos,
	}); err != nil {
		return nil, err
	}

	return &types.MsgCommitDelegationToTierResponse{PositionId: pos.Id}, nil
}

func (ms msgServer) TierDelegate(ctx context.Context, msg *types.MsgTierDelegate) (*types.MsgTierDelegateResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateDelegatePosition(ctx, pos, msg.Owner); err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.Validator)
	if err != nil {
		return nil, err
	}

	currentRatio, err := ms.updateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	newShares, err := ms.delegate(ctx, valAddr, pos.Amount)
	if err != nil {
		return nil, err
	}

	if err := ms.applyDelegationToPosition(ctx, &pos, types.Delegation{
		Validator:           msg.Validator,
		Shares:              newShares,
		BaseRewardsPerShare: currentRatio,
	}); err != nil {
		return nil, err
	}

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionDelegated{
		PositionId: pos.Id,
		TierId:     pos.TierId,
		Owner:      pos.Owner,
		Validator:  msg.Validator,
		Shares:     newShares,
	}); err != nil {
		return nil, err
	}

	return &types.MsgTierDelegateResponse{PositionId: pos.Id}, nil
}

func (ms msgServer) TierUndelegate(ctx context.Context, msg *types.MsgTierUndelegate) (*types.MsgTierUndelegateResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateUndelegatePosition(ctx, pos, msg.Owner); err != nil {
		return nil, err
	}

	pos, _, _, err = ms.claimAndRefreshPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return nil, err
	}

	completionTime, returnAmount, unbondingId, err := ms.undelegate(ctx, valAddr, pos.DelegatedShares)
	if err != nil {
		return nil, err
	}

	err = ms.setUnbondingPositionMapping(ctx, unbondingId, pos.Id)
	if err != nil {
		return nil, err
	}

	pos.UpdateAmount(returnAmount)

	srcValidator := pos.Validator
	pos.ClearDelegation()

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionUndelegated{
		PositionId:     pos.Id,
		TierId:         pos.TierId,
		Owner:          pos.Owner,
		Validator:      srcValidator,
		CompletionTime: completionTime,
	}); err != nil {
		return nil, err
	}

	return &types.MsgTierUndelegateResponse{
		CompletionTime: completionTime,
		PositionId:     pos.Id,
	}, nil
}

func (ms msgServer) TierRedelegate(ctx context.Context, msg *types.MsgTierRedelegate) (*types.MsgTierRedelegateResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateRedelegatePosition(ctx, pos, msg.Owner, msg.DstValidator); err != nil {
		return nil, err
	}

	dstValAddr, err := sdk.ValAddressFromBech32(msg.DstValidator)
	if err != nil {
		return nil, err
	}

	pos, _, _, err = ms.claimAndRefreshPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	srcValAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return nil, err
	}

	// Snapshot destination validator's ratio before new shares arrive.
	dstCurrentRatio, err := ms.updateBaseRewardsPerShare(ctx, dstValAddr)
	if err != nil {
		return nil, err
	}

	completionTime, newShares, unbondingId, err := ms.redelegate(ctx, srcValAddr, dstValAddr, pos.DelegatedShares)
	if err != nil {
		return nil, err
	}

	err = ms.setRedelegationPositionMapping(ctx, unbondingId, pos.Id)
	if err != nil {
		return nil, err
	}

	srcValidator := pos.Validator
	if err := ms.applyDelegationToPosition(ctx, &pos, types.Delegation{
		Validator:           msg.DstValidator,
		Shares:              newShares,
		BaseRewardsPerShare: dstCurrentRatio,
	}); err != nil {
		return nil, err
	}

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionRedelegated{
		PositionId:     pos.Id,
		TierId:         pos.TierId,
		Owner:          pos.Owner,
		SrcValidator:   srcValidator,
		DstValidator:   msg.DstValidator,
		NewShares:      newShares,
		CompletionTime: completionTime,
	}); err != nil {
		return nil, err
	}

	return &types.MsgTierRedelegateResponse{
		CompletionTime: completionTime,
		PositionId:     pos.Id,
	}, nil
}

func (ms msgServer) AddToTierPosition(ctx context.Context, msg *types.MsgAddToTierPosition) (*types.MsgAddToTierPositionResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateAddToPosition(ctx, pos, msg.Owner); err != nil {
		return nil, err
	}

	if err := ms.lockFunds(ctx, msg.Owner, msg.Amount); err != nil {
		return nil, err
	}

	if pos.IsDelegated() {
		pos, _, _, err = ms.claimAndRefreshPosition(ctx, pos)
		if err != nil {
			return nil, err
		}

		valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
		if err != nil {
			return nil, err
		}

		newShares, err := ms.delegate(ctx, valAddr, msg.Amount)
		if err != nil {
			return nil, err
		}

		totalShares := pos.DelegatedShares.Add(newShares)
		if err := ms.applyDelegationToPosition(ctx, &pos, types.Delegation{
			Validator:           pos.Validator,
			Shares:              totalShares,
			BaseRewardsPerShare: pos.BaseRewardsPerShare,
		}); err != nil {
			return nil, err
		}
	} else {
		pos.UpdateAmount(pos.Amount.Add(msg.Amount))
	}

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionAmountAdded{
		PositionId:  pos.Id,
		TierId:      pos.TierId,
		Owner:       pos.Owner,
		AmountAdded: msg.Amount,
		NewTotal:    pos.Amount,
	}); err != nil {
		return nil, err
	}

	return &types.MsgAddToTierPositionResponse{PositionId: pos.Id}, nil
}

func (ms msgServer) TriggerExitFromTier(ctx context.Context, msg *types.MsgTriggerExitFromTier) (*types.MsgTriggerExitFromTierResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateTriggerExit(pos, msg.Owner); err != nil {
		return nil, err
	}

	tier, err := ms.getTier(ctx, pos.TierId)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pos.TriggerExit(sdkCtx.BlockTime(), tier.ExitDuration)

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventExitTriggered{
		PositionId:   pos.Id,
		TierId:       pos.TierId,
		Owner:        pos.Owner,
		ExitUnlockAt: pos.ExitUnlockAt,
	}); err != nil {
		return nil, err
	}

	return &types.MsgTriggerExitFromTierResponse{
		ExitUnlockAt: pos.ExitUnlockAt,
		PositionId:   pos.Id,
	}, nil
}

func (ms msgServer) ClearPosition(ctx context.Context, msg *types.MsgClearPosition) (*types.MsgClearPositionResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateClearPosition(ctx, pos, msg.Owner); err != nil {
		return nil, err
	}

	pos, _, _, err = ms.claimAndRefreshPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pos.ClearExit(sdkCtx.BlockTime())

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventExitCleared{
		PositionId: pos.Id,
		TierId:     pos.TierId,
		Owner:      pos.Owner,
	}); err != nil {
		return nil, err
	}

	return &types.MsgClearPositionResponse{PositionId: pos.Id}, nil
}

func (ms msgServer) ClaimTierRewards(ctx context.Context, msg *types.MsgClaimTierRewards) (*types.MsgClaimTierRewardsResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateClaimRewards(pos, msg.Owner); err != nil {
		return nil, err
	}

	pos, baseRewards, bonusRewards, err := ms.claimAndRefreshPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	if err := ms.setPosition(ctx, pos); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventTierRewardsClaimed{
		PositionId:   pos.Id,
		TierId:       pos.TierId,
		Owner:        pos.Owner,
		BaseRewards:  baseRewards,
		BonusRewards: bonusRewards,
	}); err != nil {
		return nil, err
	}

	return &types.MsgClaimTierRewardsResponse{
		BaseRewards:  baseRewards,
		BonusRewards: bonusRewards,
		PositionId:   pos.Id,
	}, nil
}

func (ms msgServer) WithdrawFromTier(ctx context.Context, msg *types.MsgWithdrawFromTier) (*types.MsgWithdrawFromTierResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.getPosition(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.validateWithdrawFromTier(ctx, pos, msg.Owner); err != nil {
		return nil, err
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
	if err != nil {
		return nil, err
	}

	bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}

	withdrawCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, pos.Amount))

	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, withdrawCoins); err != nil {
		return nil, err
	}

	if err := ms.deletePosition(ctx, pos); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionWithdrawn{
		Position: pos,
		Amount:   withdrawCoins,
	}); err != nil {
		return nil, err
	}

	return &types.MsgWithdrawFromTierResponse{
		Amount: withdrawCoins,
	}, nil
}
