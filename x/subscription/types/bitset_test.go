package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v1/x/subscription/types"
	"github.com/stretchr/testify/require"
)

func TestBitSetBasic(t *testing.T) {
	v := types.NewBitSet()
	r, err := v.Test(0)
	require.False(t, r)
	require.NoError(t, err)

	// error cases
	err = v.Set(64)
	require.Error(t, err)
	err = v.Clear(64)
	require.Error(t, err)
	r, err = v.Test(64)
	require.False(t, r)
	require.Error(t, err)

	err = v.Set(0)
	require.NoError(t, err)
	err = v.Set(20)
	require.NoError(t, err)
	err = v.Set(63)
	require.NoError(t, err)
	r, err = v.Test(0)
	require.True(t, r)
	require.NoError(t, err)
	r, err = v.Test(20)
	require.True(t, r)
	require.NoError(t, err)
	r, err = v.Test(63)
	require.NoError(t, err)
	require.True(t, r)
	require.Equal(t, uint(3), v.Len())
	err = v.Clear(20)
	require.NoError(t, err)
	r, err = v.Test(20)
	require.False(t, r)
	require.NoError(t, err)
	require.Equal(t, uint(2), v.Len())
}

func TestBitSetIterate(t *testing.T) {
	v := types.NewBitSet()
	r, err := v.Test(0)
	require.False(t, r)
	require.NoError(t, err)
	err = v.Set(0)
	require.NoError(t, err)
	err = v.Set(20)
	require.NoError(t, err)
	err = v.Set(63)
	require.NoError(t, err)
	i, e := v.NextSet(0)
	require.Equal(t, uint(0), i)
	require.True(t, e)
	i, e = v.NextSet(i + 1)
	require.Equal(t, uint(20), i)
	require.True(t, e)
	i, e = v.NextSet(i + 1)
	require.Equal(t, uint(63), i)
	require.True(t, e)
	i, e = v.NextSet(i + 1)
	require.Equal(t, uint(0), i)
	require.False(t, e)
}
