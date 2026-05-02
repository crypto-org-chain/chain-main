package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (ms msgServer) requireAuthority(msgAuthority string) error {
	if ms.getAuthority() != msgAuthority {
		return errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msgAuthority)
	}
	return nil
}

func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if err := ms.requireAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

func (ms msgServer) AddTier(ctx context.Context, msg *types.MsgAddTier) (*types.MsgAddTierResponse, error) {
	if err := ms.requireAuthority(msg.Authority); err != nil {
		return nil, err
	}

	has, err := ms.hasTier(ctx, msg.Tier.Id)
	if err != nil {
		return nil, err
	}
	if has {
		return nil, types.ErrTierAlreadyExists
	}

	if err := ms.SetTier(ctx, msg.Tier); err != nil {
		return nil, err
	}

	if err := ms.emitTierChangedEvent(ctx, types.TierChangeAction_TIER_CHANGE_ACTION_NEW, msg.Tier); err != nil {
		return nil, err
	}

	return &types.MsgAddTierResponse{}, nil
}

func (ms msgServer) UpdateTier(ctx context.Context, msg *types.MsgUpdateTier) (*types.MsgUpdateTierResponse, error) {
	if err := ms.requireAuthority(msg.Authority); err != nil {
		return nil, err
	}

	oldTier, err := ms.getTier(ctx, msg.Tier.Id)
	if err != nil {
		return nil, err
	}

	if !oldTier.BonusApy.Equal(msg.Tier.BonusApy) {
		if err := ms.claimRewardsAndUpdateTierPositions(ctx, msg.Tier.Id); err != nil {
			return nil, err
		}
	}

	if err := ms.SetTier(ctx, msg.Tier); err != nil {
		return nil, err
	}

	if err := ms.emitTierChangedEvent(ctx, types.TierChangeAction_TIER_CHANGE_ACTION_UPDATE, msg.Tier); err != nil {
		return nil, err
	}

	return &types.MsgUpdateTierResponse{}, nil
}

func (ms msgServer) DeleteTier(ctx context.Context, msg *types.MsgDeleteTier) (*types.MsgDeleteTierResponse, error) {
	if err := ms.requireAuthority(msg.Authority); err != nil {
		return nil, err
	}

	tier, err := ms.getTier(ctx, msg.Id)
	if err != nil {
		return nil, err
	}

	if err := ms.deleteTier(ctx, msg.Id); err != nil {
		return nil, err
	}

	if err := ms.emitTierChangedEvent(ctx, types.TierChangeAction_TIER_CHANGE_ACTION_DELETE, tier); err != nil {
		return nil, err
	}

	return &types.MsgDeleteTierResponse{}, nil
}

func (ms msgServer) emitTierChangedEvent(ctx context.Context, action types.TierChangeAction, tier types.Tier) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.EventManager().EmitTypedEvent(&types.EventTierChanged{
		Action: action,
		Tier:   tier,
	})
}
