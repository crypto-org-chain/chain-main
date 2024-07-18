//go:build !rocksdb
// +build !rocksdb

package app

import (
	"errors"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
)

func setupVersionDB(
	homePath string,
	app *baseapp.BaseApp,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
	okeys map[string]*storetypes.ObjectStoreKey,
) (storetypes.RootMultiStore, error) {
	return nil, errors.New("versiondb is not supported in this binary")
}
