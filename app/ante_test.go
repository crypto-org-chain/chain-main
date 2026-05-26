package app_test

import (
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/suite"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// tx is a minimal sdk.Tx implementation sufficient for AnteHandle
// unit tests — only GetMsgs is consulted by RejectVestingTierMsgDecorator.
type tx struct {
	msgs []sdk.Msg
}

func (t tx) GetMsgs() []sdk.Msg                    { return t.msgs }
func (t tx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

type AnteTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.ChainApp
}

func TestAnteTestSuite(t *testing.T) {
	suite.Run(t, new(AnteTestSuite))
}

func (s *AnteTestSuite) SetupTest() {
	s.app = testutil.Setup(false, nil)
	s.ctx = s.app.NewContext(false).WithBlockHeader(tmproto.Header{
		Height:  1,
		ChainID: testutil.ChainID,
		Time:    time.Now().UTC(),
	})
}

// makeBaseAccount creates and persists a fresh BaseAccount, returning its
// bech32 address.
func (s *AnteTestSuite) makeBaseAccount() string {
	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	acc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, addr)
	s.app.AccountKeeper.SetAccount(s.ctx, acc)
	return addr.String()
}

// makeVestingAccount creates and persists a fresh PermanentLockedAccount
// (a concrete VestingAccount), returning its bech32 address.
func (s *AnteTestSuite) makeVestingAccount() string {
	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	// Reserve a unique account number first; otherwise the IndexerCollections
	// uniqueness constraint trips when multiple accounts default to number 0.
	base := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, addr).(*authtypes.BaseAccount)
	originalVesting := sdk.NewCoins(sdk.NewCoin("basecro", math.NewInt(1_000_000)))
	vacc, err := vestingtypes.NewPermanentLockedAccount(base, originalVesting)
	s.Require().NoError(err)
	s.app.AccountKeeper.SetAccount(s.ctx, vacc)
	return addr.String()
}

func (s *AnteTestSuite) runDecorator(ctx sdk.Context, msgs ...sdk.Msg) error {
	dec := app.NewRejectVestingTierMsgDecorator(s.app.AccountKeeper)
	noopNext := func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	}
	_, err := dec.AnteHandle(ctx, tx{msgs: msgs}, false, noopNext)
	return err
}

func (s *AnteTestSuite) TestRejectVestingTierMsgDecorator() {
	checkTxCtx := s.ctx.WithIsCheckTx(true)
	reCheckTxCtx := s.ctx.WithIsReCheckTx(true)
	deliverTxCtx := s.ctx.WithIsCheckTx(false)

	regular := s.makeBaseAccount()
	vesting := s.makeVestingAccount()

	lockTier := func(owner string) sdk.Msg {
		return &tieredrewardstypes.MsgLockTier{
			Owner:            owner,
			Id:               1,
			Amount:           math.NewInt(1_000_000),
			ValidatorAddress: "crocncl1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
		}
	}
	commit := func(delegator string) sdk.Msg {
		return &tieredrewardstypes.MsgCommitDelegationToTier{
			DelegatorAddress: delegator,
			ValidatorAddress: "crocncl1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
			Amount:           math.NewInt(1_000_000),
			Id:               1,
		}
	}
	unrelated := &banktypes.MsgSend{
		FromAddress: vesting,
		ToAddress:   regular,
		Amount:      sdk.NewCoins(sdk.NewCoin("basecro", math.NewInt(1))),
	}

	cases := []struct {
		name    string
		ctx     sdk.Context
		msgs    []sdk.Msg
		wantErr error
	}{
		{
			name:    "CheckTx + base account + MsgLockTier → allowed",
			ctx:     checkTxCtx,
			msgs:    []sdk.Msg{lockTier(regular)},
			wantErr: nil,
		},
		{
			name:    "CheckTx + vesting account + MsgLockTier → rejected",
			ctx:     checkTxCtx,
			msgs:    []sdk.Msg{lockTier(vesting)},
			wantErr: tieredrewardstypes.ErrVestingAccountNotAllowed,
		},
		{
			name:    "CheckTx + vesting account + MsgCommitDelegationToTier → rejected",
			ctx:     checkTxCtx,
			msgs:    []sdk.Msg{commit(vesting)},
			wantErr: tieredrewardstypes.ErrVestingAccountNotAllowed,
		},
		{
			name: "CheckTx + vesting account + MsgLockTier + unrelated msg → rejected",
			ctx:  checkTxCtx,
			msgs: []sdk.Msg{unrelated, lockTier(vesting)},
			// Mixed tx still fails fast on the offending message.
			wantErr: tieredrewardstypes.ErrVestingAccountNotAllowed,
		},
		{
			name:    "CheckTx + vesting account + unrelated msg only → allowed",
			ctx:     checkTxCtx,
			msgs:    []sdk.Msg{unrelated},
			wantErr: nil,
		},
		{
			// Load-bearing: DeliverTx must NEVER reject — divergence here
			// would halt consensus. If anyone removes the IsCheckTx() gate
			// this case fails immediately.
			name:    "DeliverTx + vesting account + MsgLockTier → allowed (consensus safety)",
			ctx:     deliverTxCtx,
			msgs:    []sdk.Msg{lockTier(vesting)},
			wantErr: nil,
		},
		{
			name:    "DeliverTx + vesting account + MsgCommitDelegationToTier → allowed (consensus safety)",
			ctx:     deliverTxCtx,
			msgs:    []sdk.Msg{commit(vesting)},
			wantErr: nil,
		},
		{
			// Account doesn't exist yet — decorator now treats this as a
			// hard error so the message handler doesn't have to chase it.
			name:    "CheckTx + non-existent account + MsgLockTier → rejected (invalid address)",
			ctx:     checkTxCtx,
			msgs:    []sdk.Msg{lockTier(sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String())},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name:    "CheckTx + malformed address + MsgLockTier → rejected (invalid address)",
			ctx:     checkTxCtx,
			msgs:    []sdk.Msg{lockTier("not-a-bech32-address")},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			// ReCheckTx is mempool re-validation between blocks. Defensive:
			// today the SDK guarantees IsCheckTx() returns true on ReCheckTx
			// too, but the decorator gates explicitly on both flags so this
			// behavior is locked in even if the SDK invariant changes.
			name:    "ReCheckTx + vesting account + MsgLockTier → rejected",
			ctx:     reCheckTxCtx,
			msgs:    []sdk.Msg{lockTier(vesting)},
			wantErr: tieredrewardstypes.ErrVestingAccountNotAllowed,
		},
		{
			name:    "ReCheckTx + vesting account + MsgCommitDelegationToTier → rejected",
			ctx:     reCheckTxCtx,
			msgs:    []sdk.Msg{commit(vesting)},
			wantErr: tieredrewardstypes.ErrVestingAccountNotAllowed,
		},
		{
			name:    "ReCheckTx + base account + MsgLockTier → allowed",
			ctx:     reCheckTxCtx,
			msgs:    []sdk.Msg{lockTier(regular)},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			err := s.runDecorator(tc.ctx, tc.msgs...)
			if tc.wantErr == nil {
				s.Require().NoError(err)
			} else {
				s.Require().ErrorIs(err, tc.wantErr)
			}
		})
	}
}
