//go:build rocksdb
// +build rocksdb

package app

import (
	"os"
	"path/filepath"

	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"

	storetypes "cosmossdk.io/store/types"
)

func (app *ChainApp) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
) (storetypes.MultiStore, error) {
	dataDir := filepath.Join(homePath, "data", "versiondb")
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return nil, err
	}
	versionDB, err := tsrocksdb.NewStore(dataDir)
	if err != nil {
		return nil, err
	}

	// always listen for all keys to simplify configuration
	exposedKeys := make([]storetypes.StoreKey, 0, len(keys))
	for _, key := range keys {
		exposedKeys = append(exposedKeys, key)
	}
	app.CommitMultiStore().AddListeners(exposedKeys)

	// register in app streaming manager
	app.SetStreamingManager(storetypes.StreamingManager{
		ABCIListeners: []storetypes.ABCIListener{versiondb.NewStreamingService(versionDB)},
		StopNodeOnErr: true,
	})

	delegatedStoreKeys := make(map[storetypes.StoreKey]struct{})
	for _, k := range tkeys {
		delegatedStoreKeys[k] = struct{}{}
	}
	for _, k := range memKeys {
		delegatedStoreKeys[k] = struct{}{}
	}

	verDB := versiondb.NewMultiStore(app.CommitMultiStore(), versionDB, keys, delegatedStoreKeys)
	app.SetQueryMultiStore(verDB)
	return verDB, nil
}
