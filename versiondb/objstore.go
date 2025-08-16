//go:build objstore
// +build objstore

package versiondb

import "cosmossdk.io/store/types"

// GetObjKVStore implements `MultiStore` interface
func (s *MultiStore) GetObjKVStore(storeKey types.StoreKey) types.ObjKVStore {
	return s.GetStore(storeKey).(types.ObjKVStore)
}
