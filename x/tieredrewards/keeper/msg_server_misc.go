package keeper

import (
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (ms msgServer) FundTierPool(ctx context.Context, msg *types.MsgFundTierPool) (*types.MsgFundTierPoolResponse, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	for _, coin := range msg.Amount {
		if coin.Denom != bondDenom {
			return nil, errors.Wrapf(types.ErrInvalidAmount, "fund amount must use bond denom %s only", bondDenom)
		}
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
