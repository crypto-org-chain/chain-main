package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		name     string
		genState *types.GenesisState
		wantErr  bool
	}{
		{
			name:     "default",
			genState: types.DefaultGenesisState(),
			wantErr:  false,
		},
		{
			"valid genesis",
			&types.GenesisState{
				PortId: "portidone",
			},
			false,
		},
		{
			"invalid client",
			&types.GenesisState{
				PortId: "(INVALIDPORT)",
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.genState.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("GenesisState.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
