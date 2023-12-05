package app

import (
	newsdkerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	ibcante "github.com/cosmos/ibc-go/v5/modules/core/ante"
	"github.com/cosmos/ibc-go/v5/modules/core/keeper"
	nfttypes "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

// HandlerOptions extend the SDK's AnteHandler options by requiring the IBC
// channel keeper.
type HandlerOptions struct {
	ante.HandlerOptions

	IBCKeeper *keeper.Keeper
}

func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return nil, newsdkerrors.Wrap(sdkerrors.ErrLogic, "sign mode handler is required for ante builder")
	}

	sigGasConsumer := options.SigGasConsumer
	if sigGasConsumer == nil {
		sigGasConsumer = ante.DefaultSigVerificationGasConsumer
	}

	anteDecorators := []sdk.AnteDecorator{
		ante.NewSetUpContextDecorator(), // outermost AnteDecorator. SetUpContext must be called first
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		NewValidateMsgTransferDecorator(),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		ante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, options.TxFeeChecker),
		// SetPubKeyDecorator must be called before all signature verification decorators
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, sigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}

const (
	// values chosen arbitrarily
	MaxClassIDLength      = 256
	MaxTokenIds           = 256
	MaxTokenIDLength      = 256
	MaximumReceiverLength = 2048
)

// ValidateMsgTransferDecorator is a temporary decorator that limit the field length of MsgTransfer message.
type ValidateMsgTransferDecorator struct{}

func NewValidateMsgTransferDecorator() ValidateMsgTransferDecorator {
	return ValidateMsgTransferDecorator{}
}

func (vtd ValidateMsgTransferDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if !ctx.IsCheckTx() {
		return next(ctx, tx, simulate)
	}

	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		transfer, ok := msg.(*nfttypes.MsgTransfer)
		if !ok {
			continue
		}

		if len(transfer.ClassId) > MaxClassIDLength {
			return ctx, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "class id length must be less than %d", MaxClassIDLength)
		}

		if len(transfer.TokenIds) > MaxClassIDLength {
			return ctx, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "token id length must be less than %d", MaxClassIDLength)
		}

		for _, tokenID := range transfer.TokenIds {
			if len(tokenID) > MaxTokenIDLength {
				return ctx, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "token id length must be less than %d", MaxTokenIDLength)
			}
		}

		if len(transfer.Receiver) > MaximumReceiverLength {
			return ctx, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "receiver length must be less than %d", MaximumReceiverLength)
		}
	}

	return next(ctx, tx, simulate)
}
