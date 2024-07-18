//go:build !rocksdb
// +build !rocksdb

package app

import (
	"errors"

	storetypes "cosmossdk.io/store/types"
)

func (app *ChainApp) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
	okeys map[string]*storetypes.ObjectStoreKey,
) (storetypes.RootMultiStore, error) {
	return nil, errors.New("versiondb is not supported in this binary")
}
