package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.MsgServer = msgServer{}

// msgServer is a wrapper of Keeper.
type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the tieredrewards MsgServer interface.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

func (ms msgServer) LockTier(ctx context.Context, msg *types.MsgLockTier) (*types.MsgLockTierResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	tier, err := ms.Keeper.Tiers.Get(ctx, msg.Id)
	if err != nil {
		return nil, err
	}

	err = ms.Keeper.ValidateNewPosition(ctx, tier, msg.Amount)
	if err != nil {
		return nil, err
	}

	if err := ms.Keeper.LockFunds(ctx, msg.Owner, msg.Amount); err != nil {
		return nil, err
	}

	var delegation *types.Delegation
	if msg.ValidatorAddress != "" {
		valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		currentRatio, err := ms.Keeper.UpdateBaseRewardsPerShare(ctx, valAddr)
		if err != nil {
			return nil, err
		}
		shares, err := ms.Keeper.Delegate(ctx, valAddr, msg.Amount)
		if err != nil {
			return nil, err
		}
		delegation = &types.Delegation{
			Validator:           msg.ValidatorAddress,
			Shares:              shares,
			BaseRewardsPerShare: currentRatio,
		}
	}

	pos, err := ms.Keeper.CreatePosition(ctx, msg.Owner, tier, msg.Amount, delegation, msg.TriggerExitImmediately)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionCreated{
		Position: pos,
	}); err != nil {
		return nil, err
	}

	return &types.MsgLockTierResponse{}, nil
}

func (ms msgServer) CommitDelegationToTier(ctx context.Context, msg *types.MsgCommitDelegationToTier) (*types.MsgCommitDelegationToTierResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	tier, err := ms.Keeper.Tiers.Get(ctx, msg.Id)
	if err != nil {
		return nil, err
	}

	err = ms.Keeper.ValidateNewPosition(ctx, tier, msg.Amount)
	if err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}

	currentRatio, err := ms.Keeper.UpdateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	shares, err := ms.Keeper.TransferDelegation(ctx, *msg)
	if err != nil {
		return nil, err
	}

	delegation := &types.Delegation{
		Validator:           msg.ValidatorAddress,
		Shares:              shares,
		BaseRewardsPerShare: currentRatio,
	}

	pos, err := ms.Keeper.CreatePosition(ctx, msg.DelegatorAddress, tier, msg.Amount, delegation, msg.TriggerExitImmediately)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventDelegationCommitted{
		CommittedDelegation: types.CommittedDelegation{
			DelegatorAddress: msg.DelegatorAddress,
			ValidatorAddress: msg.ValidatorAddress,
			Amount:           msg.Amount,
		},
		Position: pos,
	}); err != nil {
		return nil, err
	}

	return &types.MsgCommitDelegationToTierResponse{}, nil
}

func (ms msgServer) TierDelegate(ctx context.Context, msg *types.MsgTierDelegate) (*types.MsgTierDelegateResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.Keeper.Positions.Get(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	if err := ms.Keeper.ValidateDelegatePosition(ctx, pos, msg.Owner); err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.Validator)
	if err != nil {
		return nil, err
	}

	currentRatio, err := ms.Keeper.UpdateBaseRewardsPerShare(ctx, valAddr)
	if err != nil {
		return nil, err
	}

	// only accept whole position amount delegate, no partial delegation
	newShares, err := ms.Keeper.Delegate(ctx, valAddr, pos.Amount)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pos.WithDelegation(types.Delegation{
		Validator:           msg.Validator,
		Shares:              newShares,
		BaseRewardsPerShare: currentRatio,
	}, sdkCtx.BlockTime())

	if err := ms.Keeper.SetPosition(ctx, pos); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventPositionDelegated{
		PositionId: pos.Id,
		TierId:     pos.TierId,
		Owner:      pos.Owner,
		Validator:  msg.Validator,
		Shares:     newShares,
	}); err != nil {
		return nil, err
	}

	return &types.MsgTierDelegateResponse{}, nil
}

func (ms msgServer) TierUndelegate(ctx context.Context, msg *types.MsgTierUndelegate) (*types.MsgTierUndelegateResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.Keeper.Positions.Get(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	err = ms.Keeper.ValidateUndelegatePosition(ctx, pos, msg.Owner)
	if err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return nil, err
	}

	if err := ms.Keeper.ClaimRewardsForPositions(ctx, valAddr, []types.Position{pos}); err != nil {
		return nil, err
	}

	// Re-fetch position after ClaimRewardsForPositions (it calls SetPosition internally).
	pos, err = ms.Keeper.Positions.Get(ctx, pos.Id)
	if err != nil {
		return nil, err
	}

	completionTime, unbondingId, err := ms.Keeper.Undelegate(ctx, valAddr, pos.DelegatedShares)
	if err != nil {
		return nil, err
	}

	if unbondingId > 0 {
		if err := ms.Keeper.UnbondingIdToPositionId.Set(ctx, unbondingId, pos.Id); err != nil {
			return nil, err
		}
	}

	srcValidator := pos.Validator
	pos.ClearDelegation()

	if err := ms.Keeper.SetPosition(ctx, pos); err != nil {
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
	}, nil
}

func (ms msgServer) TierRedelegate(ctx context.Context, msg *types.MsgTierRedelegate) (*types.MsgTierRedelegateResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	pos, err := ms.Keeper.Positions.Get(ctx, msg.PositionId)
	if err != nil {
		return nil, err
	}

	err = ms.Keeper.ValidateRedelegatePosition(ctx, pos, msg.Owner, msg.DstValidator)
	if err != nil {
		return nil, err
	}

	srcValAddr, err := sdk.ValAddressFromBech32(pos.Validator)
	if err != nil {
		return nil, err
	}

	dstValAddr, err := sdk.ValAddressFromBech32(msg.DstValidator)
	if err != nil {
		return nil, err
	}

	if err := ms.Keeper.ClaimRewardsForPositions(ctx, srcValAddr, []types.Position{pos}); err != nil {
		return nil, err
	}

	// Re-fetch position after claiming.
	pos, err = ms.Keeper.Positions.Get(ctx, pos.Id)
	if err != nil {
		return nil, err
	}

	// Snapshot destination validator's ratio before new shares arrive.
	dstCurrentRatio, err := ms.Keeper.UpdateBaseRewardsPerShare(ctx, dstValAddr)
	if err != nil {
		return nil, err
	}

	completionTime, newShares, unbondingId, err := ms.Keeper.Redelegate(ctx, srcValAddr, dstValAddr, pos.DelegatedShares)
	if err != nil {
		return nil, err
	}

	if unbondingId > 0 {
		if err := ms.Keeper.UnbondingIdToPositionId.Set(ctx, unbondingId, pos.Id); err != nil {
			return nil, err
		}
	}

	srcValidator := pos.Validator
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pos.WithDelegation(types.Delegation{
		Validator:           msg.DstValidator,
		Shares:              newShares,
		BaseRewardsPerShare: dstCurrentRatio,
	}, sdkCtx.BlockTime())

	if err := ms.Keeper.SetPosition(ctx, pos); err != nil {
		return nil, err
	}

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
	}, nil
}
