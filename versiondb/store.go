package versiondb

import (
	"io"
	"time"

	"cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
)

const StoreTypeVersionDB = 100

var _ types.KVStore = (*Store)(nil)

// Store Implements types.KVStore
type Store struct {
	store    VersionStore
	storeKey types.StoreKey
	version  *int64
}

func NewKVStore(store VersionStore, storeKey types.StoreKey, version *int64) *Store {
	return &Store{store, storeKey, version}
}

// Implements Store.
func (st *Store) GetStoreType() types.StoreType {
	// should have effect, just define an unique indentifier, don't be conflicts with cosmos-sdk's builtin ones.
	return StoreTypeVersionDB
}

// Implements Store.
func (st *Store) CacheWrap() types.CacheWrap {
	return cachekv.NewStore(st)
}

// Implements types.KVStore.
func (st *Store) Get(key []byte) []byte {
	defer telemetry.MeasureSince(time.Now(), "store", "versiondb", "get")
	value, err := st.store.GetAtVersion(st.storeKey.Name(), key, st.version)
	if err != nil {
		panic(err)
	}
	return value
}

// Implements types.KVStore.
func (st *Store) Has(key []byte) (exists bool) {
	defer telemetry.MeasureSince(time.Now(), "store", "versiondb", "has")
	has, err := st.store.HasAtVersion(st.storeKey.Name(), key, st.version)
	if err != nil {
		panic(err)
	}
	return has
}

// Implements types.KVStore.
func (st *Store) Iterator(start, end []byte) types.Iterator {
	itr, err := st.store.IteratorAtVersion(st.storeKey.Name(), start, end, st.version)
	if err != nil {
		panic(err)
	}
	return itr
}

// Implements types.KVStore.
func (st *Store) ReverseIterator(start, end []byte) types.Iterator {
	itr, err := st.store.ReverseIteratorAtVersion(st.storeKey.Name(), start, end, st.version)
	if err != nil {
		panic(err)
	}
	return itr
}

// Implements types.KVStore.
func (st *Store) Set(key, value []byte) {
	panic("write operation is not supported")
}

// Implements types.KVStore.
func (st *Store) Delete(key []byte) {
	panic("write operation is not supported")
}

func (st *Store) CacheWrapWithTrace(w io.Writer, tc types.TraceContext) types.CacheWrap {
	panic("not implemented")
}
