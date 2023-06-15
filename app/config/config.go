package config

import "github.com/crypto-org-chain/cronos/memiavl"

const DefaultCacheSize = 1000

type MemIAVLConfig struct {
	// Enable defines if the memiavl should be enabled.
	Enable bool `mapstructure:"enable"`
	// ZeroCopy defines if the memiavl should return slices pointing to mmap-ed buffers directly (zero-copy),
	// the zero-copied slices must not be retained beyond current block's execution.
	ZeroCopy bool `mapstructure:"zero-copy"`
	// AsyncCommitBuffer defines the size of asynchronous commit queue, this greatly improve block catching-up
	// performance, -1 means synchronous commit.
	AsyncCommitBuffer int `mapstructure:"async-commit-buffer"`
	// SnapshotKeepRecent defines what many old snapshots (excluding the latest one) to keep after new snapshots are taken.
	SnapshotKeepRecent uint32 `mapstructure:"snapshot-keep-recent"`
	// SnapshotInterval defines the block interval the memiavl snapshot is taken, default to 1000.
	SnapshotInterval uint32 `mapstructure:"snapshot-interval"`
	// CacheSize defines the size of the cache for each memiavl store.
	CacheSize int `mapstructure:"cache-size"`
}

func DefaultMemIAVLConfig() MemIAVLConfig {
	return MemIAVLConfig{
		CacheSize:          DefaultCacheSize,
		SnapshotInterval:   memiavl.DefaultSnapshotInterval,
		ZeroCopy:           true,
		SnapshotKeepRecent: 1,
	}
}
