//go:build !rocksdb

package app

import (
	"errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

func (app *ChainApp) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
) (storetypes.MultiStore, error) {
	return nil, errors.New("versiondb is not supported in this binary")
}
