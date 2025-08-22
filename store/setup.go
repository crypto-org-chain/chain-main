package store

import (
	"path/filepath"

	"cosmossdk.io/log"
	"github.com/spf13/cast"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/crypto-org-chain/cronos/store/rootmulti"
)

const (
	FlagMemIAVL             = "memiavl.enable"
	FlagAsyncCommitBuffer   = "memiavl.async-commit-buffer"
	FlagZeroCopy            = "memiavl.zero-copy"
	FlagSnapshotKeepRecent  = "memiavl.snapshot-keep-recent"
	FlagSnapshotInterval    = "memiavl.snapshot-interval"
	FlagCacheSize           = "memiavl.cache-size"
	FlagSnapshotWriterLimit = "memiavl.snapshot-writer-limit"
)

// SetupMemIAVL insert the memiavl setter in front of baseapp options, so that
// the default rootmulti store is replaced by memiavl store,
func SetupMemIAVL(
	logger log.Logger,
	homePath string,
	appOpts servertypes.AppOptions,
	sdk46Compact bool,
	supportExportNonSnapshotVersion bool,
	cacheSize int,
	baseAppOptions []func(*baseapp.BaseApp),
) []func(*baseapp.BaseApp) {
	if cast.ToBool(appOpts.Get(FlagMemIAVL)) {
		opts := memiavl.Options{
			AsyncCommitBuffer:   cast.ToInt(appOpts.Get(FlagAsyncCommitBuffer)),
			ZeroCopy:            cast.ToBool(appOpts.Get(FlagZeroCopy)),
			SnapshotKeepRecent:  cast.ToUint32(appOpts.Get(FlagSnapshotKeepRecent)),
			SnapshotInterval:    cast.ToUint32(appOpts.Get(FlagSnapshotInterval)),
			CacheSize:           cacheSize,
			SnapshotWriterLimit: cast.ToInt(appOpts.Get(FlagSnapshotWriterLimit)),
		}

		if opts.ZeroCopy {
			// it's unsafe to cache zero-copied byte slices without copying them
			sdk.SetAddrCacheEnabled(false)
		}

		// cms must be overridden before the other options, because they may use the cms,
		// make sure the cms aren't be overridden by the other options later on.
		baseAppOptions = append([]func(*baseapp.BaseApp){setMemIAVL(homePath, logger, opts, sdk46Compact, supportExportNonSnapshotVersion)}, baseAppOptions...)
	}

	return baseAppOptions
}

func setMemIAVL(homePath string, logger log.Logger, opts memiavl.Options, sdk46Compact bool, supportExportNonSnapshotVersion bool) func(*baseapp.BaseApp) {
	return func(bapp *baseapp.BaseApp) {
		// trigger state-sync snapshot creation by memiavl
		opts.TriggerStateSyncExport = func(height int64) {
			go bapp.SnapshotManager().SnapshotIfApplicable(height)
		}
		cms := rootmulti.NewStore(filepath.Join(homePath, "data", "memiavl.db"), logger, sdk46Compact, supportExportNonSnapshotVersion)
		cms.SetMemIAVLOptions(opts)
		bapp.SetCMS(cms)
	}
}
