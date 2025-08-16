package versiondb

import (
	"fmt"
	"io"
	"sync"

	"cosmossdk.io/store/cachemulti"
	"cosmossdk.io/store/types"
)

var _ types.MultiStore = (*MultiStore)(nil)

// MultiStore wraps `VersionStore` to implement `MultiStore` interface.
type MultiStore struct {
	versionDB VersionStore
	stores    map[types.StoreKey]types.KVStore

	// transient/memory/object stores, they are delegated to the parent
	delegatedStoreKeys map[types.StoreKey]struct{}

	// proxy the calls for transient or mem stores to the parent
	parent types.MultiStore

	traceWriter       io.Writer
	traceContext      types.TraceContext
	traceContextMutex sync.Mutex
}

// NewMultiStore returns a new versiondb `MultiStore`.
func NewMultiStore(
	parent types.MultiStore,
	versionDB VersionStore,
	storeKeys map[string]*types.KVStoreKey,
	delegatedStoreKeys map[types.StoreKey]struct{},
) *MultiStore {
	stores := make(map[types.StoreKey]types.KVStore, len(storeKeys))
	for _, k := range storeKeys {
		stores[k] = NewKVStore(versionDB, k, nil)
	}
	return &MultiStore{
		versionDB:          versionDB,
		stores:             stores,
		parent:             parent,
		delegatedStoreKeys: delegatedStoreKeys,
	}
}

// GetStoreType implements `MultiStore` interface.
func (s *MultiStore) GetStoreType() types.StoreType {
	return types.StoreTypeMulti
}

// cacheMultiStore branch out the multistore.
func (s *MultiStore) cacheMultiStore(version *int64) types.CacheMultiStore {
	stores := make(map[types.StoreKey]types.CacheWrapper, len(s.delegatedStoreKeys)+len(s.stores))
	for k := range s.delegatedStoreKeys {
		stores[k] = types.CacheWrapper(s.parent.GetStore(k))
	}
	for k := range s.stores {
		if version == nil {
			stores[k] = s.stores[k]
		} else {
			stores[k] = NewKVStore(s.versionDB, k, version)
		}
	}
	return cachemulti.NewStore(nil, stores, nil, s.traceWriter, s.getTracingContext())
}

// CacheMultiStore implements `MultiStore` interface
func (s *MultiStore) CacheMultiStore() types.CacheMultiStore {
	return s.cacheMultiStore(nil)
}

// CacheMultiStoreWithVersion implements `MultiStore` interface
func (s *MultiStore) CacheMultiStoreWithVersion(version int64) (types.CacheMultiStore, error) {
	return s.cacheMultiStore(&version), nil
}

// CacheWrap implements CacheWrapper/MultiStore/CommitStore.
func (s *MultiStore) CacheWrap() types.CacheWrap {
	return s.CacheMultiStore().(types.CacheWrap)
}

// GetStore implements `MultiStore` interface
func (s *MultiStore) GetStore(storeKey types.StoreKey) types.Store {
	if store, ok := s.stores[storeKey]; ok {
		return store
	}
	if _, ok := s.delegatedStoreKeys[storeKey]; ok {
		// delegate the transient/memory/object stores to real cms
		return s.parent.GetStore(storeKey)
	}
	panic(fmt.Errorf("store key %s is not registered", storeKey.Name()))
}

// GetKVStore implements `MultiStore` interface
func (s *MultiStore) GetKVStore(storeKey types.StoreKey) types.KVStore {
	return s.GetStore(storeKey).(types.KVStore)
}

// SetTracer sets the tracer for the MultiStore that the underlying
// stores will utilize to trace operations. A MultiStore is returned.
func (s *MultiStore) SetTracer(w io.Writer) types.MultiStore {
	s.traceWriter = w
	return s
}

// SetTracingContext updates the tracing context for the MultiStore by merging
// the given context with the existing context by key. Any existing keys will
// be overwritten. It is implied that the caller should update the context when
// necessary between tracing operations. It returns a modified MultiStore.
func (s *MultiStore) SetTracingContext(tc types.TraceContext) types.MultiStore {
	s.traceContextMutex.Lock()
	defer s.traceContextMutex.Unlock()
	s.traceContext = s.traceContext.Merge(tc)

	return s
}

func (s *MultiStore) getTracingContext() types.TraceContext {
	s.traceContextMutex.Lock()
	defer s.traceContextMutex.Unlock()

	if s.traceContext == nil {
		return nil
	}

	ctx := types.TraceContext{}
	for k, v := range s.traceContext {
		ctx[k] = v
	}

	return ctx
}

// TracingEnabled returns if tracing is enabled for the MultiStore.
func (s *MultiStore) TracingEnabled() bool {
	return s.traceWriter != nil
}

// LatestVersion returns the latest version saved in versiondb
func (s *MultiStore) LatestVersion() int64 {
	version, err := s.versionDB.GetLatestVersion()
	if err != nil {
		panic(err)
	}
	return version
}

// Close will flush the versiondb
func (s *MultiStore) Close() error {
	return s.versionDB.Flush()
}

// CacheWrapWithTrace is kept to build with upstream sdk.
func (s *MultiStore) CacheWrapWithTrace(w io.Writer, tc types.TraceContext) types.CacheWrap {
	panic("not implemented")
}
