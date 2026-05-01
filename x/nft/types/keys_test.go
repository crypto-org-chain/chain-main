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
	// Valid: 64-char hex hash (SHA256)
	hash := "27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
	keyDenom := []byte("ibc/" + hash + "/testtokenid")

	denomID, tokenID, err := types.SplitKeyDenom(keyDenom)

	require.NoError(t, err)
	require.Equal(t, "ibc/"+hash, denomID)
	require.Equal(t, "testtokenid", tokenID)
}

func TestSplitKeyDenomWithIBCInvalidHashLength(t *testing.T) {
	// Hash too short — should be rejected
	keyDenom := []byte("ibc/shorthash/testtokenid")

	_, _, err := types.SplitKeyDenom(keyDenom)

	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong KeyDenom")
}

func TestSplitKeyDenomWithIBCNoTokenID(t *testing.T) {
	// No delimiter after hash — missing tokenID
	keyDenom := []byte("ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2")

	_, _, err := types.SplitKeyDenom(keyDenom)

	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong KeyDenom")
}

func TestSplitKeyDenomNonIBCWithSlash(t *testing.T) {
	// Non-IBC key with slashes: first segment is denomID, rest is tokenID
	keyDenom := []byte("port/channel/classid/testtokenid")

	denomID, tokenID, err := types.SplitKeyDenom(keyDenom)

	require.NoError(t, err)
	require.Equal(t, "port", denomID)
	require.Equal(t, "channel/classid/testtokenid", tokenID)
}

func TestSplitKeyDenomNoDelimiter(t *testing.T) {
	keyDenom := []byte("nodeliminatall")

	_, _, err := types.SplitKeyDenom(keyDenom)

	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong KeyDenom")
}

func TestSplitKeyOwnerWithNonIBC(t *testing.T) {
	addr := sdk.AccAddress([]byte("cosmos1testaddr______"))
	denomID := "testdenomid"

	t.Run("simple token ID", func(t *testing.T) {
		tokenID := "testtokenid"
		key := types.KeyOwner(addr, denomID, tokenID)

		gotAddr, gotDenom, gotToken, err := types.SplitKeyOwner(key)
		require.NoError(t, err)
		require.Equal(t, addr, gotAddr)
		require.Equal(t, denomID, gotDenom)
		require.Equal(t, tokenID, gotToken)
	})

	t.Run("token ID with slashes", func(t *testing.T) {
		tokenID := "collection/series/42"
		key := types.KeyOwner(addr, denomID, tokenID)

		gotAddr, gotDenom, gotToken, err := types.SplitKeyOwner(key)
		require.NoError(t, err)
		require.Equal(t, addr, gotAddr)
		require.Equal(t, denomID, gotDenom)
		require.Equal(t, tokenID, gotToken)
	})
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
