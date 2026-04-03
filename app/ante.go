package app

import (
	"strings"

	ibcante "github.com/cosmos/ibc-go/v10/modules/core/ante"
	"github.com/cosmos/ibc-go/v10/modules/core/keeper"
	nfttypes "github.com/crypto-org-chain/chain-main/v8/x/nft-transfer/types"

	newsdkerrors "cosmossdk.io/errors"
	circuitante "cosmossdk.io/x/circuit/ante"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
)

// HandlerOptions extend the SDK's AnteHandler options by requiring the IBC
// channel keeper.
type HandlerOptions struct {
	ante.HandlerOptions

	IBCKeeper     *keeper.Keeper
	CircuitKeeper circuitante.CircuitBreaker
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
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
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

	handler := sdk.ChainAnteDecorators(anteDecorators...)
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		newCtx, err := handler(ctx, tx, simulate)
		if err != nil && strings.Contains(err.Error(), "not found") && newsdkerrors.IsOf(err, sdkerrors.ErrUnknownAddress) {
			return newCtx, newsdkerrors.Wrap(err, "send fund to create this account, this is not a keyring-backend issue")
		}
		return newCtx, err
	}, nil
}

const (
	// MaxClassIDLength values chosen arbitrarily
	MaxClassIDLength      = 2048
	MaxTokenIds           = 256
	MaxTokenIDLength      = 2048
	MaximumReceiverLength = 2048
)

// ValidateMsgTransferDecorator is a temporary decorator that limit the field length of MsgTransfer message.
type ValidateMsgTransferDecorator struct{}

func NewValidateMsgTransferDecorator() ValidateMsgTransferDecorator {
	return ValidateMsgTransferDecorator{}
}

func (vtd ValidateMsgTransferDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		transferMsg, ok := msg.(*nfttypes.MsgTransfer)
		if !ok {
			continue
		}
		classID := transferMsg.ClassId
		if len(classID) > MaxClassIDLength {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "class id length must be less than %d", MaxClassIDLength)
		}
		tokenIds := transferMsg.TokenIds
		if len(tokenIds) > MaxTokenIds {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "token ids length must be less than %d", MaxTokenIds)
		}
		for _, tokenID := range tokenIds {
			if len(tokenID) > MaxTokenIDLength {
				return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "token id length must be less than %d", MaxTokenIDLength)
			}
		}
		if len(transferMsg.Receiver) > MaximumReceiverLength {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "receiver length must be less than %d", MaximumReceiverLength)
		}
	}
	return next(ctx, tx, simulate)
}
