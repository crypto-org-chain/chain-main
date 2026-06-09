// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Cronos.org (licensed under the Apache License, Version 2.0)
package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/nft/types"
	"github.com/stretchr/testify/require"
)

func TestValidateGenesis(t *testing.T) {
	mkState := func(id, name string) types.GenesisState {
		return *types.NewGenesisState([]types.Collection{
			{Denom: types.NewDenom(id, name, "", "", address)},
		})
	}

	testCases := []struct {
		name    string
		state   types.GenesisState
		wantErr bool
	}{
		{"free-form name accepted", mkState("validid", "Free Form Name!"), false},
		{"ibc voucher id accepted", mkState("ibc/"+
			"0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF", "ibc-voucher"), false},
		{"empty name rejected", mkState("validid", "   "), true},
		{"non-hex ibc id rejected", mkState("ibc/"+
			"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"[:64], "name"), true},
		{"invalid id rejected", mkState("Bad ID", "name"), true},
		{"too-short id rejected", mkState("ab", "name"), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateGenesis(tc.state)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
