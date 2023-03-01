//go:build !rocksdb
// +build !rocksdb

package opendb

import (
	"path/filepath"

	dbm "github.com/tendermint/tm-db"
)

func OpenDB(home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	return dbm.NewDB("application", backendType, dataDir)
}

// OpenReadOnlyDB opens rocksdb backend in read-only mode.
func OpenReadOnlyDB(home string, backendType dbm.BackendType) (dbm.DB, error) {
	return OpenDB(home, backendType)
}
