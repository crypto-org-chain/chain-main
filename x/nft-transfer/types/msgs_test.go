package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

var (
	sender   = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	receiver = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
)

func TestMsgTransfer_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     *types.MsgTransfer
		wantErr bool
	}{
		{"valid msg", types.NewMsgTransfer("nft-transfer", "channel-1", "cryptoCat", []string{"kitty"}, sender, receiver, clienttypes.NewHeight(1, 1), 1), false},
		{"invalid msg with port", types.NewMsgTransfer("@nft-transfer", "channel-1", "cryptoCat", []string{"kitty"}, sender, receiver, clienttypes.NewHeight(1, 1), 1), true},
		{"invalid msg with channel", types.NewMsgTransfer("nft-transfer", "@channel-1", "cryptoCat", []string{"kitty"}, sender, receiver, clienttypes.NewHeight(1, 1), 1), true},
		{"invalid msg with class", types.NewMsgTransfer("nft-transfer", "channel-1", "", []string{"kitty"}, sender, receiver, clienttypes.NewHeight(1, 1), 1), true},
		{"invalid msg with token_id", types.NewMsgTransfer("nft-transfer", "channel-1", "cryptoCat", []string{""}, sender, receiver, clienttypes.NewHeight(1, 1), 1), true},
		{"invalid msg with sender", types.NewMsgTransfer("nft-transfer", "channel-1", "cryptoCat", []string{"kitty"}, "", receiver, clienttypes.NewHeight(1, 1), 1), true},
		{"invalid msg with receiver", types.NewMsgTransfer("nft-transfer", "channel-1", "cryptoCat", []string{"kitty"}, sender, "", clienttypes.NewHeight(1, 1), 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.msg.ValidateBasic(); (err != nil) != tt.wantErr {
				t.Errorf("MsgTransfer.ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
