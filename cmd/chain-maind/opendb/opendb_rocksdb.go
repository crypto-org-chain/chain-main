//go:build rocksdb
// +build rocksdb

package opendb

import (
	"path/filepath"
	"runtime"

	"github.com/linxGnu/grocksdb"
	dbm "github.com/tendermint/tm-db"
)

func OpenDB(home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	if backendType == dbm.RocksDBBackend {
		// customize rocksdb options
		db, err := grocksdb.OpenDb(NewRocksdbOptions(false), filepath.Join(dataDir, "application.db"))
		if err != nil {
			return nil, err
		}
		ro := grocksdb.NewDefaultReadOptions()
		wo := grocksdb.NewDefaultWriteOptions()
		woSync := grocksdb.NewDefaultWriteOptions()
		woSync.SetSync(true)
		return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
	}

	return dbm.NewDB("application", backendType, dataDir)
}

// OpenReadOnlyDB opens rocksdb backend in read-only mode.
func OpenReadOnlyDB(home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	if backendType == dbm.RocksDBBackend {
		// customize rocksdb options
		db, err := grocksdb.OpenDbForReadOnly(NewRocksdbOptions(false), filepath.Join(dataDir, "application.db"), false)
		if err != nil {
			return nil, err
		}

		ro := grocksdb.NewDefaultReadOptions()
		wo := grocksdb.NewDefaultWriteOptions()
		woSync := grocksdb.NewDefaultWriteOptions()
		woSync.SetSync(true)
		return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
	}

	return dbm.NewDB("application", backendType, dataDir)
}

func NewRocksdbOptions(sstFileWriter bool) *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.IncreaseParallelism(runtime.NumCPU())
	opts.OptimizeLevelStyleCompaction(512 * 1024 * 1024)
	opts.SetTargetFileSizeMultiplier(2)

	// block based table options
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()

	// 1G block cache
	bbto.SetBlockCache(grocksdb.NewLRUCache(1 << 30))

	// http://rocksdb.org/blog/2021/12/29/ribbon-filter.html
	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))

	// partition index
	// http://rocksdb.org/blog/2017/05/12/partitioned-index-filter.html
	bbto.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	bbto.SetPartitionFilters(true)

	// hash index is better for iavl tree which mostly do point lookup.
	bbto.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)

	opts.SetBlockBasedTableFactory(bbto)

	// in iavl tree, we almost always query existing keys
	opts.SetOptimizeFiltersForHits(true)

	// heavier compression option at bottommost level,
	// 110k dict bytes is default in zstd library,
	// train bytes is recommended to be set at 100x dict bytes.
	opts.SetBottommostCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	compressOpts.Level = 12
	if !sstFileWriter {
		compressOpts.MaxDictBytes = 110 * 1024
		opts.SetBottommostCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes*100, true)
	}
	opts.SetBottommostCompressionOptions(compressOpts, true)
	return opts
}
