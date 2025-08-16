package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"cosmossdk.io/errors"
	"github.com/alitto/pond"
	gogotypes "github.com/cosmos/gogoproto/types"
	"github.com/cosmos/iavl/keyformat"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	storetypes "cosmossdk.io/store/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"

	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/crypto-org-chain/cronos/versiondb/extsort"
)

const (
	int64Size = 8
	int32Size = 4

	storeKeyPrefix   = "s/k:%s/"
	latestVersionKey = "s/latest"
	commitInfoKeyFmt = "s/%d" // s/<version>

	// We creates the temporary sst files in the target database to make sure the file renaming is cheap in ingestion
	// part.
	StoreSSTFileName = "tmp-%s-%d.sst"

	PipelineBufferSize         = 1024
	DefaultSorterChunkSizeIAVL = 64 * 1024 * 1024
)

var (
	nodeKeyFormat   = keyformat.NewKeyFormat('n', memiavl.SizeHash)              // n<hash>
	rootKeyFormat   = keyformat.NewKeyFormat('r', int64Size)                     // r<version>
	nodeKeyV1Format = keyformat.NewFastPrefixFormatter('s', int64Size+int32Size) // s<version><nonce>
)

func RestoreAppDBCmd(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore-app-db snapshot-dir application.db",
		Short: "Restore `application.db` from memiavl snapshots",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sstFileSizeTarget, err := cmd.Flags().GetUint64(flagSSTFileSize)
			if err != nil {
				return err
			}
			sorterChunkSize, err := cmd.Flags().GetUint64(flagSorterChunkSize)
			if err != nil {
				return err
			}
			concurrency, err := cmd.Flags().GetInt(flagConcurrency)
			if err != nil {
				return err
			}
			sdk64Compact, err := cmd.Flags().GetBool(flagSDK64Compact)
			if err != nil {
				return err
			}
			stores, err := GetStoresOrDefault(cmd, opts.DefaultStores)
			if err != nil {
				return err
			}

			snapshotDir := args[0]
			iavlDir := args[1]
			if err := os.MkdirAll(iavlDir, os.ModePerm); err != nil {
				return err
			}

			// load the snapshots and compute commit info first
			var lastestVersion int64
			var storeInfos []storetypes.StoreInfo
			if sdk64Compact {
				// https://github.com/cosmos/cosmos-sdk/issues/14916
				storeInfos = append(storeInfos, storetypes.StoreInfo{Name: capabilitytypes.MemStoreKey, CommitId: storetypes.CommitID{}})
			}
			snapshots := make([]*memiavl.Snapshot, len(stores))
			for i, store := range stores {
				path := filepath.Join(snapshotDir, store)
				snapshot, err := memiavl.OpenSnapshot(path)
				if err != nil {
					return errors.Wrapf(err, "open snapshot fail: %s", path)
				}
				snapshots[i] = snapshot

				tree := memiavl.NewFromSnapshot(snapshot, true, 0)
				commitID := lastCommitID(tree)
				storeInfos = append(storeInfos, storetypes.StoreInfo{
					Name:     store,
					CommitId: commitID,
				})

				if commitID.Version > lastestVersion {
					lastestVersion = commitID.Version
				}
			}
			commitInfo := buildCommitInfo(storeInfos, lastestVersion)

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 0)
			defer pool.StopAndWait()

			group, _ := pool.GroupContext(context.Background())
			for i := 0; i < len(stores); i++ {
				// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
				store := stores[i]
				snapshot := snapshots[i]
				group.Submit(func() error {
					defer snapshot.Close()

					sstWriter := newIAVLSSTFileWriter(opts.AppRocksDBOptions)
					defer sstWriter.Destroy()

					return oneStore(sstWriter, store, snapshot, iavlDir, sstFileSizeTarget, sorterChunkSize)
				})
			}

			if err := group.Wait(); err != nil {
				return errors.Wrap(err, "worker pool wait fail")
			}

			// collect the sst files
			entries, err := os.ReadDir(iavlDir)
			if err != nil {
				return errors.Wrapf(err, "read directory fail: %s", iavlDir)
			}
			sstFiles := make([]string, 0, len(entries))
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "tmp-") {
					sstFiles = append(sstFiles, filepath.Join(iavlDir, name))
				}
			}

			// sst files ingestion
			ingestOpts := grocksdb.NewDefaultIngestExternalFileOptions()
			defer ingestOpts.Destroy()
			ingestOpts.SetMoveFiles(true)

			db, err := grocksdb.OpenDb(opts.AppRocksDBOptions(false), iavlDir)
			if err != nil {
				return errors.Wrap(err, "open iavl db fail")
			}
			defer db.Close()

			if err := db.IngestExternalFile(sstFiles, ingestOpts); err != nil {
				return errors.Wrap(err, "ingset sst files fail")
			}

			// write the metadata part separately, because it overlaps with the other sst files
			if err := writeMetadata(db, &commitInfo); err != nil {
				return errors.Wrap(err, "write metadata fail")
			}

			fmt.Printf("version: %d, app hash: %X\n", commitInfo.Version, commitInfo.Hash())
			return nil
		},
	}

	cmd.Flags().Uint64(flagSSTFileSize, DefaultSSTFileSize, "the target sst file size, note the actual file size may be larger because sst files must be split on different key names")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().Uint64(flagSorterChunkSize, DefaultSorterChunkSizeIAVL, "uncompressed chunk size for external sorter, it decides the peak ram usage, on disk it'll be snappy compressed")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")
	cmd.Flags().Bool(flagSDK64Compact, false, "Should the app hash calculation be compatible with cosmos-sdk v0.46 and earlier")

	return cmd
}

// oneStore process a single store, can run in parallel with other stores,
func oneStore(sstWriter *grocksdb.SSTFileWriter, store string, snapshot *memiavl.Snapshot, sstDir string, sstFileSizeTarget, sorterChunkSize uint64) error {
	prefix := []byte(fmt.Sprintf(storeKeyPrefix, store))

	inputChan, outputChan := extsort.Spawn(sstDir, extsort.Options{
		MaxChunkSize:      int64(sorterChunkSize),
		LesserFunc:        compareSorterNode,
		SnappyCompression: true,
	}, PipelineBufferSize)

	err := snapshot.ScanNodes(func(node memiavl.PersistedNode) error {
		bz, err := encodeSorterNode(node)
		if err != nil {
			return err
		}
		inputChan <- bz
		return nil
	})
	close(inputChan)
	if err != nil {
		return err
	}

	sstSeq := 0
	openNextFile := func() error {
		sstFileName := filepath.Join(sstDir, fmt.Sprintf(StoreSSTFileName, store, sstSeq))
		if err := sstWriter.Open(sstFileName); err != nil {
			return errors.Wrapf(err, "open sst file fail: %s", sstFileName)
		}
		sstSeq++
		return nil
	}

	if err := openNextFile(); err != nil {
		return err
	}
	for item := range outputChan {
		hash := item[:memiavl.SizeHash]
		value := item[memiavl.SizeHash:]
		key := cloneAppend(prefix, nodeKeyFormat.Key(hash))

		if err := sstWriter.Put(key, value); err != nil {
			return errors.Wrap(err, "sst write node fail")
		}

		if sstWriter.FileSize() >= sstFileSizeTarget {
			if err := sstWriter.Finish(); err != nil {
				return errors.Wrap(err, "sst writer finish fail")
			}
			if err := openNextFile(); err != nil {
				return err
			}
		}
	}

	// root record, it use empty slice for root hash of empty tree
	rootKey := cloneAppend(prefix, rootKeyFormat.Key(int64(snapshot.Version())))
	var rootHash []byte
	if !snapshot.IsEmpty() {
		rootHash = snapshot.RootNode().Hash()
	}
	if err := sstWriter.Put(rootKey, rootHash); err != nil {
		return errors.Wrap(err, "sst write root fail")
	}

	if err := sstWriter.Finish(); err != nil {
		return errors.Wrap(err, "sst writer finish fail")
	}

	return nil
}

// writeMetadata writes the rootmulti commit info and latest version to the db
func writeMetadata(db *grocksdb.DB, cInfo *storetypes.CommitInfo) error {
	writeOpts := grocksdb.NewDefaultWriteOptions()

	bz, err := cInfo.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal CommitInfo fail")
	}

	cInfoKey := fmt.Sprintf(commitInfoKeyFmt, cInfo.Version)
	if err := db.Put(writeOpts, []byte(cInfoKey), bz); err != nil {
		return err
	}

	bz, err = gogotypes.StdInt64Marshal(cInfo.Version)
	if err != nil {
		return err
	}

	return db.Put(writeOpts, []byte(latestVersionKey), bz)
}

func newIAVLSSTFileWriter(rocksdbOpts func(bool) *grocksdb.Options) *grocksdb.SSTFileWriter {
	envOpts := grocksdb.NewDefaultEnvOptions()
	return grocksdb.NewSSTFileWriter(envOpts, rocksdbOpts(true))
}

// encodeNode encodes the node in the same way as the existing iavl implementation.
func encodeNode(w io.Writer, node memiavl.PersistedNode) error {
	var buf [binary.MaxVarintLen64]byte

	height := node.Height()
	n := binary.PutVarint(buf[:], int64(height))
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}
	n = binary.PutVarint(buf[:], node.Size())
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}
	n = binary.PutVarint(buf[:], int64(node.Version()))
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}

	// Unlike writeHashBytes, key is written for inner nodes.
	if err := memiavl.EncodeBytes(w, node.Key()); err != nil {
		return err
	}

	if height == 0 {
		if err := memiavl.EncodeBytes(w, node.Value()); err != nil {
			return err
		}
	} else {
		if err := memiavl.EncodeBytes(w, node.Left().Hash()); err != nil {
			return err
		}
		if err := memiavl.EncodeBytes(w, node.Right().Hash()); err != nil {
			return err
		}
	}

	return nil
}

func encodeSorterNode(node memiavl.PersistedNode) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.Write(node.Hash()); err != nil {
		return nil, err
	}
	if err := encodeNode(&buf, node); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// compareSorterNode compare the hash part
func compareSorterNode(a, b []byte) bool {
	return bytes.Compare(a[:memiavl.SizeHash], b[:memiavl.SizeHash]) == -1
}
