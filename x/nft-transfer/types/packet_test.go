package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

func TestNonFungibleTokenPacketData_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		packet  types.NonFungibleTokenPacketData
		wantErr bool
	}{
		{
			name:    "valid packet",
			packet:  types.NonFungibleTokenPacketData{"cryptoCat", "uri", []string{"kitty"}, []string{"kitty_uri"}, sender, receiver},
			wantErr: false,
		},
		{
			name:    "invalid packet with empty classID",
			packet:  types.NonFungibleTokenPacketData{"", "uri", []string{"kitty"}, []string{"kitty_uri"}, sender, receiver},
			wantErr: true,
		},
		{
			name:    "invalid packet with empty tokenIds",
			packet:  types.NonFungibleTokenPacketData{"cryptoCat", "uri", []string{}, []string{"kitty_uri"}, sender, receiver},
			wantErr: true,
		},
		{
			name:    "invalid packet with empty tokenUris",
			packet:  types.NonFungibleTokenPacketData{"cryptoCat", "uri", []string{"kitty"}, []string{}, sender, receiver},
			wantErr: true,
		},
		{
			name:    "invalid packet with empty sender",
			packet:  types.NonFungibleTokenPacketData{"cryptoCat", "uri", []string{"kitty"}, []string{}, "", receiver},
			wantErr: true,
		},
		{
			name:    "invalid packet with empty receiver",
			packet:  types.NonFungibleTokenPacketData{"cryptoCat", "uri", []string{"kitty"}, []string{}, sender, receiver},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.packet.ValidateBasic(); (err != nil) != tt.wantErr {
				t.Errorf("NonFungibleTokenPacketData.ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
