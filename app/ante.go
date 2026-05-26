package app

import (
	ibcante "github.com/cosmos/ibc-go/v10/modules/core/ante"
	"github.com/cosmos/ibc-go/v10/modules/core/keeper"
	nfttypes "github.com/crypto-org-chain/chain-main/v8/x/nft-transfer/types"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	newsdkerrors "cosmossdk.io/errors"
	circuitante "cosmossdk.io/x/circuit/ante"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
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
		NewRejectVestingTierMsgDecorator(options.AccountKeeper),
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
	// avoid breaking consensus
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
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "class id length must be less than %d", MaxClassIDLength)
		}

		if len(transfer.TokenIds) > MaxTokenIds {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "token id length must be less than %d", MaxTokenIds)
		}

		for _, tokenID := range transfer.TokenIds {
			if len(tokenID) > MaxTokenIDLength {
				return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "token id length must be less than %d", MaxTokenIDLength)
			}
		}

		if len(transfer.Receiver) > MaximumReceiverLength {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "receiver length must be less than %d", MaximumReceiverLength)
		}
	}

	return next(ctx, tx, simulate)
}

// RejectVestingTierMsgDecorator rejects MsgLockTier and MsgCommitDelegationToTier
// from vesting accounts at mempool admission only. The check is gated on
// CheckTx so DeliverTx behavior is unchanged — this is a non-consensus,
// node-local filter that can be rolled out to validators independently. Once
// every validator runs the new binary, no proposer accepts these messages
// from vesting accounts and the bypass is permanently closed without a
// coordinated upgrade.
type RejectVestingTierMsgDecorator struct {
	ak ante.AccountKeeper
}

func NewRejectVestingTierMsgDecorator(ak ante.AccountKeeper) RejectVestingTierMsgDecorator {
	return RejectVestingTierMsgDecorator{ak: ak}
}

func (d RejectVestingTierMsgDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if !ctx.IsCheckTx() {
		return next(ctx, tx, simulate)
	}

	for _, msg := range tx.GetMsgs() {
		var addr string
		switch m := msg.(type) {
		case *tieredrewardstypes.MsgLockTier:
			addr = m.Owner
		case *tieredrewardstypes.MsgCommitDelegationToTier:
			addr = m.DelegatorAddress
		default:
			continue
		}

		accAddr, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address: %s", addr)
		}

		acc := d.ak.GetAccount(ctx, accAddr)
		if acc == nil {
			return ctx, newsdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "account not found: %s", accAddr.String())
		}

		if _, ok := acc.(sdkvesting.VestingAccount); ok {
			return ctx, tieredrewardstypes.ErrVestingAccountNotAllowed
		}
	}

	return next(ctx, tx, simulate)
}
