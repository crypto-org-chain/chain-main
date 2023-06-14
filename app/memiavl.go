package app

import (
	"path/filepath"

	"github.com/spf13/cast"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"

	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/crypto-org-chain/cronos/store/rootmulti"
)

const (
	FlagMemIAVL            = "memiavl.enable"
	FlagAsyncCommitBuffer  = "memiavl.async-commit-buffer"
	FlagZeroCopy           = "memiavl.zero-copy"
	FlagSnapshotKeepRecent = "memiavl.snapshot-keep-recent"
	FlagSnapshotInterval   = "memiavl.snapshot-interval"
	FlagCacheSize          = "memiavl.cache-size"
)

func SetupMemIAVL(logger log.Logger, homePath string, appOpts servertypes.AppOptions, baseAppOptions []func(*baseapp.BaseApp)) []func(*baseapp.BaseApp) {
	if cast.ToBool(appOpts.Get(FlagMemIAVL)) {
		// cms must be overridden before the other options, because they may use the cms,
		// make sure the cms aren't be overridden by the other options later on.
		cms := rootmulti.NewStore(filepath.Join(homePath, "data", "memiavl.db"), logger)
		cms.SetMemIAVLOptions(memiavl.Options{
			AsyncCommitBuffer:  cast.ToInt(appOpts.Get(FlagAsyncCommitBuffer)),
			ZeroCopy:           cast.ToBool(appOpts.Get(FlagZeroCopy)),
			SnapshotKeepRecent: cast.ToUint32(appOpts.Get(FlagSnapshotKeepRecent)),
			SnapshotInterval:   cast.ToUint32(appOpts.Get(FlagSnapshotInterval)),
			// make sure a few queryable states even with pruning="nothing", needed for ibc relayer to work.
			MinQueryStates: 3,
			CacheSize:      cast.ToInt(appOpts.Get(FlagCacheSize)),
		})
		baseAppOptions = append([]func(*baseapp.BaseApp){setCMS(cms)}, baseAppOptions...)
	}

	return baseAppOptions
}

func setCMS(cms storetypes.CommitMultiStore) func(*baseapp.BaseApp) {
	return func(bapp *baseapp.BaseApp) { bapp.SetCMS(cms) }
}
