package versiondb

import (
	"cosmossdk.io/store/types"
)

type Iterator interface {
	types.Iterator

	Timestamp() []byte
}

// VersionStore is a versioned storage of a flat key-value pairs.
// it don't need to support merkle proof, so could be implemented in a much more efficient way.
// `nil` version means the latest version.
type VersionStore interface {
	GetAtVersion(storeKey string, key []byte, version *int64) ([]byte, error)
	HasAtVersion(storeKey string, key []byte, version *int64) (bool, error)
	IteratorAtVersion(storeKey string, start, end []byte, version *int64) (Iterator, error)
	ReverseIteratorAtVersion(storeKey string, start, end []byte, version *int64) (Iterator, error)
	GetLatestVersion() (int64, error)

	// Persist the change set of a block,
	// the `changeSet` should be ordered by (storeKey, key),
	// the version should be latest version plus one.
	PutAtVersion(version int64, changeSet []*types.StoreKVPair) error

	// Import the initial state of the store
	Import(version int64, ch <-chan ImportEntry) error

	// Flush wal logs, and make the changes persistent,
	// mainly for rocksdb version upgrade, sometimes the wal format is not compatible.
	Flush() error
}

type ImportEntry struct {
	StoreKey string
	Key      []byte
	Value    []byte
}
