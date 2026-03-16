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
