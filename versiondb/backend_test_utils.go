package versiondb

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/store/types"
)

var (
	key1       = []byte("key1")
	value1     = []byte("value1")
	key1Subkey = []byte("key1/subkey")
)

func SetupTestDB(t *testing.T, store VersionStore) {
	changeSets := [][]*types.StoreKVPair{
		{
			{StoreKey: "evm", Key: []byte("delete-in-block2"), Value: []byte("1")},
			{StoreKey: "evm", Key: []byte("re-add-in-block3"), Value: []byte("1")},
			{StoreKey: "evm", Key: []byte("z-genesis-only"), Value: []byte("2")},
			{StoreKey: "evm", Key: []byte("modify-in-block2"), Value: []byte("1")},
			{StoreKey: "staking", Key: []byte("key1"), Value: []byte("value1")},
			{StoreKey: "staking", Key: []byte("key1/subkey"), Value: []byte("value1")},
		},
		{
			{StoreKey: "evm", Key: []byte("re-add-in-block3"), Delete: true},
			{StoreKey: "evm", Key: []byte("add-in-block1"), Value: []byte("1")},
			{StoreKey: "staking", Key: []byte("key1"), Delete: true},
		},
		{
			{StoreKey: "evm", Key: []byte("add-in-block2"), Value: []byte("1")},
			{StoreKey: "evm", Key: []byte("delete-in-block2"), Delete: true},
			{StoreKey: "evm", Key: []byte("modify-in-block2"), Value: []byte("2")},
			{StoreKey: "evm", Key: []byte("key2"), Delete: true},
			{StoreKey: "staking", Key: []byte("key1"), Value: []byte("value2")},
		},
		{
			{StoreKey: "evm", Key: []byte("re-add-in-block3"), Value: []byte("2")},
		},
		{
			{StoreKey: "evm", Key: []byte("re-add-in-block3"), Delete: true},
		},
	}
	for i, changeSet := range changeSets {
		require.NoError(t, store.PutAtVersion(int64(i), changeSet))
	}
}

func Run(t *testing.T, storeCreator func() VersionStore) {
	testBasics(t, storeCreator())
	testIterator(t, storeCreator())
	testHeightInFuture(t, storeCreator())

	// test delete in genesis, noop
	store := storeCreator()
	err := store.PutAtVersion(0, []*types.StoreKVPair{
		{StoreKey: "evm", Key: []byte{1}, Delete: true},
	})
	require.NoError(t, err)
}

func testBasics(t *testing.T, store VersionStore) {
	var v int64

	SetupTestDB(t, store)

	value, err := store.GetAtVersion("evm", []byte("z-genesis-only"), nil)
	require.NoError(t, err)
	require.Equal(t, []byte("2"), value)

	v = 4
	ok, err := store.HasAtVersion("evm", []byte("z-genesis-only"), &v)
	require.NoError(t, err)
	require.True(t, ok)
	value, err = store.GetAtVersion("evm", []byte("z-genesis-only"), &v)
	require.NoError(t, err)
	require.Equal(t, value, []byte("2"))

	value, err = store.GetAtVersion("evm", []byte("re-add-in-block3"), nil)
	require.NoError(t, err)
	require.Empty(t, value)

	ok, err = store.HasAtVersion("staking", key1, nil)
	require.NoError(t, err)
	require.True(t, ok)

	value, err = store.GetAtVersion("staking", key1, nil)
	require.NoError(t, err)
	require.Equal(t, value, []byte("value2"))

	v = 2
	value, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.Equal(t, value, []byte("value2"))

	ok, err = store.HasAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.True(t, ok)

	v = 0
	value, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.Equal(t, value, []byte("value1"))

	v = 1
	value, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.Empty(t, value)

	ok, err = store.HasAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.False(t, ok)

	v = 0
	value, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.Equal(t, value1, value)
	value, err = store.GetAtVersion("staking", key1Subkey, &v)
	require.NoError(t, err)
	require.Equal(t, value1, value)
}

type kvPair struct {
	Key   []byte
	Value []byte
}

func testIterator(t *testing.T, store VersionStore) {
	SetupTestDB(t, store)

	expItems := [][]kvPair{
		{
			kvPair{[]byte("delete-in-block2"), []byte("1")},
			kvPair{[]byte("modify-in-block2"), []byte("1")},
			kvPair{[]byte("re-add-in-block3"), []byte("1")},
			kvPair{[]byte("z-genesis-only"), []byte("2")},
		},
		{
			kvPair{[]byte("add-in-block1"), []byte("1")},
			kvPair{[]byte("delete-in-block2"), []byte("1")},
			kvPair{[]byte("modify-in-block2"), []byte("1")},
			kvPair{[]byte("z-genesis-only"), []byte("2")},
		},
		{
			kvPair{[]byte("add-in-block1"), []byte("1")},
			kvPair{[]byte("add-in-block2"), []byte("1")},
			kvPair{[]byte("modify-in-block2"), []byte("2")},
			kvPair{[]byte("z-genesis-only"), []byte("2")},
		},
		{
			kvPair{[]byte("add-in-block1"), []byte("1")},
			kvPair{[]byte("add-in-block2"), []byte("1")},
			kvPair{[]byte("modify-in-block2"), []byte("2")},
			kvPair{[]byte("re-add-in-block3"), []byte("2")},
			kvPair{[]byte("z-genesis-only"), []byte("2")},
		},
		{
			kvPair{[]byte("add-in-block1"), []byte("1")},
			kvPair{[]byte("add-in-block2"), []byte("1")},
			kvPair{[]byte("modify-in-block2"), []byte("2")},
			kvPair{[]byte("z-genesis-only"), []byte("2")},
		},
	}
	for i, exp := range expItems {
		t.Run(fmt.Sprintf("block-%d", i), func(t *testing.T) {
			v := int64(i)
			it, err := store.IteratorAtVersion("evm", nil, nil, &v)
			require.NoError(t, err)
			require.Equal(t, exp, consumeIterator(it))

			it, err = store.ReverseIteratorAtVersion("evm", nil, nil, &v)
			require.NoError(t, err)
			actual := consumeIterator(it)
			require.Equal(t, len(exp), len(actual))
			require.Equal(t, reversed(exp), actual)
		})
	}

	it, err := store.IteratorAtVersion("evm", nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, expItems[len(expItems)-1], consumeIterator(it))

	it, err = store.ReverseIteratorAtVersion("evm", nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, reversed(expItems[len(expItems)-1]), consumeIterator(it))

	// with start parameter
	v := int64(2)
	it, err = store.IteratorAtVersion("evm", []byte("\xff"), nil, &v)
	require.NoError(t, err)
	require.Empty(t, consumeIterator(it))
	it, err = store.ReverseIteratorAtVersion("evm", nil, []byte("\x00"), &v)
	require.NoError(t, err)
	require.Empty(t, consumeIterator(it))

	it, err = store.IteratorAtVersion("evm", []byte("modify-in-block2"), nil, &v)
	require.NoError(t, err)
	require.Equal(t, expItems[2][len(expItems[2])-2:], consumeIterator(it))

	it, err = store.ReverseIteratorAtVersion("evm", nil, []byte("mp"), &v)
	require.NoError(t, err)
	require.Equal(t,
		reversed(expItems[2][:len(expItems[2])-1]),
		consumeIterator(it),
	)

	it, err = store.ReverseIteratorAtVersion("evm", nil, []byte("modify-in-block3"), &v)
	require.NoError(t, err)
	require.Equal(t,
		reversed(expItems[2][:len(expItems[2])-1]),
		consumeIterator(it),
	)

	// delete the last key, cover some edge cases
	v = int64(len(expItems))
	err = store.PutAtVersion(
		v,
		[]*types.StoreKVPair{
			{StoreKey: "evm", Key: []byte("z-genesis-only"), Delete: true},
		},
	)
	require.NoError(t, err)
	it, err = store.IteratorAtVersion("evm", nil, nil, &v)
	require.NoError(t, err)
	require.Equal(t,
		expItems[v-1][:len(expItems[v-1])-1],
		consumeIterator(it),
	)
	v--
	it, err = store.IteratorAtVersion("evm", nil, nil, &v)
	require.NoError(t, err)
	require.Equal(t,
		expItems[v],
		consumeIterator(it),
	)
}

func testHeightInFuture(t *testing.T, store VersionStore) {
	SetupTestDB(t, store)

	latest, err := store.GetLatestVersion()
	require.NoError(t, err)

	// query in future height is the same as latest height.
	v := latest + 1
	_, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	_, err = store.HasAtVersion("staking", key1, &v)
	require.NoError(t, err)
	_, err = store.IteratorAtVersion("staking", nil, nil, &v)
	require.NoError(t, err)
	_, err = store.ReverseIteratorAtVersion("staking", nil, nil, &v)
	require.NoError(t, err)
}

func consumeIterator(it dbm.Iterator) []kvPair {
	var result []kvPair
	for ; it.Valid(); it.Next() {
		result = append(result, kvPair{it.Key(), it.Value()})
	}
	it.Close()
	return result
}

// reversed clone and reverse the slice
func reversed[S ~[]E, E any](s S) []E {
	r := make([]E, len(s))
	for i, j := 0, len(s)-1; i <= j; i, j = i+1, j-1 {
		r[i], r[j] = s[j], s[i]
	}
	return r
}
