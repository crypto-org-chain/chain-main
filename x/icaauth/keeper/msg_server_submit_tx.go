package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/chain-main/v4/x/icaauth/types"
)

func (k msgServer) SubmitTx(goCtx context.Context, msg *types.MsgSubmitTx) (*types.MsgSubmitTxResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	msgs, err := msg.GetMessages()
	if err != nil {
		return nil, err
	}

	minTimeoutDuration := k.MinTimeoutDuration(ctx)

	err = k.DoSubmitTx(ctx, msg.ConnectionId, msg.Owner, msgs, msg.CalculateTimeoutDuration(minTimeoutDuration))
	if err != nil {
		return nil, err
	}

	return &types.MsgSubmitTxResponse{}, nil
}
