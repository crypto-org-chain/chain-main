package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/nft/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestSplitKeyDenomWithoutIBC(t *testing.T) {
	keyDenom := []byte("testdenomid/testtokenid")

	denomID, tokenID, err := types.SplitKeyDenom(keyDenom)

	require.NoError(t, err)
	require.Equal(t, "testdenomid", denomID)
	require.Equal(t, "testtokenid", tokenID)
}

func TestSplitKeyDenomWithIBC(t *testing.T) {
	keyDenom := []byte("ibc/testdenomid/testtokenid")

	denomID, tokenID, err := types.SplitKeyDenom(keyDenom)

	require.NoError(t, err)
	require.Equal(t, "ibc/testdenomid", denomID)
	require.Equal(t, "testtokenid", tokenID)
}

func TestSplitKeyOwnerWithIBC(t *testing.T) {
	addr := sdk.AccAddress([]byte("cosmos1testaddr______"))
	ibcDenom := "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"

	t.Run("simple token ID", func(t *testing.T) {
		tokenID := "testtokenid"
		key := types.KeyOwner(addr, ibcDenom, tokenID)

		gotAddr, gotDenom, gotToken, err := types.SplitKeyOwner(key)
		require.NoError(t, err)
		require.Equal(t, addr, gotAddr)
		require.Equal(t, ibcDenom, gotDenom)
		require.Equal(t, tokenID, gotToken)
	})

	t.Run("token ID with slashes", func(t *testing.T) {
		tokenID := "collection/series/42"
		key := types.KeyOwner(addr, ibcDenom, tokenID)

		gotAddr, gotDenom, gotToken, err := types.SplitKeyOwner(key)
		require.NoError(t, err)
		require.Equal(t, addr, gotAddr)
		require.Equal(t, ibcDenom, gotDenom)
		require.Equal(t, tokenID, gotToken)
	})
}
