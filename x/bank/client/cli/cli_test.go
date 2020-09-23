package cli_test

import (
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/crypto-com/chain-main/test"
	banktestutil "github.com/crypto-com/chain-main/x/bank/client/cli/testutil"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	cfg := network.DefaultConfig()
	cfg.NumValidators = 1

	s.cfg = cfg
	s.network = network.New(s.T(), cfg)

	_, err := s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestNewSendTxCmdGenOnly() {
	val := s.network.Validators[0]

	clientCtx := val.ClientCtx

	// ctx := context.Background()
	// ctx = context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)

	fakeCoinMappings := map[string]sdk.Coin{
		"1node0supertoken": sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
		"1superstake":      sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
	}
	fakeCoinParser := test.NewFakeCoinParser(fakeCoinMappings)

	from := val.Address
	to := val.Address
	amount := sdk.NewCoins(
		sdk.NewCoin(fmt.Sprintf("%ssupertoken", val.Moniker), sdk.NewInt(1)),
		sdk.NewCoin("superstake", sdk.NewInt(1)),
	)
	expectedAmount := sdk.NewCoins(
		sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
		sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
	)
	args := []string{
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=true", flags.FlagGenerateOnly),
	}

	bz, err := banktestutil.MsgSendExec(clientCtx, fakeCoinParser, from, to, amount, args...)
	s.Require().NoError(err)
	tx, err := s.cfg.TxConfig.TxJSONDecoder()(bz.Bytes())
	s.Require().NoError(err)
	s.Require().Equal([]sdk.Msg{types.NewMsgSend(from, to, expectedAmount)}, tx.GetMsgs())
}

func (s *IntegrationTestSuite) TestNewSendTxCmd() {
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		from, to       sdk.AccAddress
		amount         sdk.Coins
		expectedAmount sdk.Coins
		args           []string
		expectErr      bool
		respType       proto.Message
		expectedCode   uint32
	}{
		{
			"valid transaction",
			val.Address,
			val.Address,
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%ssupertoken", val.Moniker), sdk.NewInt(1)),
				sdk.NewCoin("superstake", sdk.NewInt(1)),
			),
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			),
			[]string{
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false,
			&sdk.TxResponse{},
			0,
		},
		{
			"not enough fees",
			val.Address,
			val.Address,
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%ssupertoken", val.Moniker), sdk.NewInt(1)),
				sdk.NewCoin("superstake", sdk.NewInt(1)),
			),
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			),
			[]string{
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(1))).String()),
			},
			false,
			&sdk.TxResponse{},
			sdkerrors.ErrInsufficientFee.ABCICode(),
		},
		{
			"not enough gas",
			val.Address,
			val.Address,
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%ssupertoken", val.Moniker), sdk.NewInt(1)),
				sdk.NewCoin("superstake", sdk.NewInt(1)),
			),
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			),
			[]string{
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
				"--gas=10",
			},
			false,
			&sdk.TxResponse{},
			sdkerrors.ErrOutOfGas.ABCICode(),
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			clientCtx := val.ClientCtx

			fakeCoinMappings := map[string]sdk.Coin{
				"1node0supertoken": sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				"0superstake":      sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(0)),
				"1superstake":      sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			}
			fakeCoinParser := test.NewFakeCoinParser(fakeCoinMappings)

			bz, err := banktestutil.MsgSendExec(clientCtx, fakeCoinParser, tc.from, tc.to, tc.amount, tc.args...)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				s.Require().NoError(clientCtx.JSONMarshaler.UnmarshalJSON(bz.Bytes(), tc.respType), bz.String())
				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code)
			}
		})
	}
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
