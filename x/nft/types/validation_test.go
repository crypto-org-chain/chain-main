// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021-present Cronos.org (licensed under the Apache License, Version 2.0)
package types_test

import (
	"strings"
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/nft/types"
	"github.com/stretchr/testify/require"
)

func TestValidateDenomIDWithIBC(t *testing.T) {
	hash64 := "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"

	testCases := []struct {
		name    string
		denomID string
		wantErr bool
	}{
		{"plain id accepted", "validid", false},
		{"ibc hex hash accepted", "ibc/" + hash64, false},
		{"ibc lowercase hex accepted", "ibc/" + strings.ToLower(hash64), false},
		{"ibc non-hex suffix rejected", "ibc/" + strings.Repeat("g", 64), true},
		{"ibc wrong length rejected", "ibc/" + hash64[:63], true},
		{"invalid plain id rejected", "Bad ID", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateDenomIDWithIBC(tc.denomID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
