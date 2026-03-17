package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msg.Authority)
	}

	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

func (ms msgServer) AddTier(ctx context.Context, msg *types.MsgAddTier) (*types.MsgAddTierResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msg.Authority)
	}

	has, err := ms.HasTier(ctx, msg.Tier.Id)
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
	if ms.GetAuthority() != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msg.Authority)
	}

	has, err := ms.HasTier(ctx, msg.Tier.Id)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, collections.ErrNotFound
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
	if ms.GetAuthority() != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msg.Authority)
	}

	tier, err := ms.Tiers.Get(ctx, msg.Id)
	if err != nil {
		return nil, err
	}

	if err := ms.Keeper.DeleteTier(ctx, msg.Id); err != nil {
		return nil, err
	}

	if err := ms.emitTierChangedEvent(ctx, types.TierChangeAction_TIER_CHANGE_ACTION_DELETE, tier); err != nil {
		return nil, err
	}

	return &types.MsgDeleteTierResponse{}, nil
}

func (ms msgServer) FundTierPool(ctx context.Context, msg *types.MsgFundTierPool) (*types.MsgFundTierPoolResponse, error) {
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	if !params.IsAuthorizedFunder(msg.Depositor) {
		return nil, errors.Wrapf(types.ErrUnauthorizedFunder, "address %s is not in pool_funders whitelist", msg.Depositor)
	}

	if !msg.Amount.IsValid() || msg.Amount.IsZero() {
		return nil, errors.Wrap(types.ErrInvalidAmount, "fund amount must be valid and non-zero")
	}

	depositor, err := sdk.AccAddressFromBech32(msg.Depositor)
	if err != nil {
		return nil, err
	}

	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, depositor, types.RewardsPoolName, msg.Amount); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventTierPoolFunded{
		Depositor: msg.Depositor,
		Amount:    msg.Amount,
	}); err != nil {
		return nil, err
	}

	return &types.MsgFundTierPoolResponse{}, nil
}

func (ms msgServer) emitTierChangedEvent(ctx context.Context, action types.TierChangeAction, tier types.Tier) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.EventManager().EmitTypedEvent(&types.EventTierChanged{
		Action: action,
		Tier:   tier,
	})
}
