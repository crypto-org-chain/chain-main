package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alitto/pond"
	"github.com/cosmos/iavl"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/versiondb/extsort"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

const (
	SSTFileExtension       = ".sst"
	DefaultSSTFileSize     = 128 * 1024 * 1024
	DefaultSorterChunkSize = 256 * 1024 * 1024

	// SizeKeyLength is the number of bytes used to encode key length in sort payload
	SizeKeyLength = 4
)

func BuildVersionDBSSTCmd(defaultStores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-versiondb-sst changeSetDir sstDir",
		Short: "Build versiondb rocksdb sst files from changesets, different stores can run in parallel, the sst files are used to rebuild versiondb later",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sstFileSize, err := cmd.Flags().GetUint64(flagSSTFileSize)
			if err != nil {
				return err
			}

			sorterChunkSize, err := cmd.Flags().GetInt64(flagSorterChunkSize)
			if err != nil {
				return err
			}
			concurrency, err := cmd.Flags().GetInt(flagConcurrency)
			if err != nil {
				return err
			}
			stores, err := GetStoresOrDefault(cmd, defaultStores)
			if err != nil {
				return err
			}

			changeSetDir := args[0]
			sstDir := args[1]

			if err := os.MkdirAll(sstDir, os.ModePerm); err != nil {
				return err
			}

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 0)
			defer pool.StopAndWait()

			group, _ := pool.GroupContext(context.Background())
			for _, store := range stores {
				// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
				store := store
				group.Submit(func() error {
					return convertSingleStore(store, changeSetDir, sstDir, sstFileSize, sorterChunkSize)
				})
			}

			return group.Wait()
		},
	}

	cmd.Flags().Uint64(flagSSTFileSize, DefaultSSTFileSize, "the target sst file size, note the actual file size may be larger because sst files must be split on different key names")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().Int64(flagSorterChunkSize, DefaultSorterChunkSize, "uncompressed chunk size for external sorter, it decides the peak ram usage, on disk it'll be snappy compressed")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")

	return cmd
}

// convertSingleStore handles a single store, can run in parallel with other stores,
// it starts extra goroutines for parallel pipeline.
func convertSingleStore(store string, changeSetDir, sstDir string, sstFileSize uint64, sorterChunkSize int64) error {
	csFiles, err := scanChangeSetFiles(changeSetDir, store)
	if err != nil {
		return err
	}
	if len(csFiles) == 0 {
		return nil
	}

	prefix := []byte(fmt.Sprintf(tsrocksdb.StorePrefixTpl, store))
	isEmpty := true

	inputChan, outputChan := extsort.Spawn(sstDir, extsort.Options{
		MaxChunkSize:      sorterChunkSize,
		LesserFunc:        compareSorterItem,
		DeltaEncoding:     true,
		SnappyCompression: true,
	}, PipelineBufferSize)

	for _, file := range csFiles {
		if err = withChangeSetFile(file.FileName, func(reader Reader) error {
			_, err := IterateChangeSets(reader, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
				for _, pair := range changeSet.Pairs {
					inputChan <- encodeSorterItem(uint64(version), pair)
					isEmpty = false
				}
				return true, nil
			})

			return err
		}); err != nil {
			break
		}
	}
	close(inputChan)
	if err != nil {
		return err
	}

	if isEmpty {
		// SSTFileWriter don't support writing empty files, so we stop early here.
		return nil
	}

	sstWriter := newSSTFileWriter()
	defer sstWriter.Destroy()

	sstSeq := 0
	openNextFile := func() error {
		if err := sstWriter.Open(filepath.Join(sstDir, sstFileName(store, sstSeq))); err != nil {
			return err
		}
		sstSeq++
		return nil
	}

	if err := openNextFile(); err != nil {
		return err
	}

	var lastKey []byte
	for item := range outputChan {
		ts, pair := decodeSorterItem(item)

		// Only breakup sst file when the user-key(without timestamp) is different,
		// because the rocksdb ingestion logic checks for overlap in keys without the timestamp part currently.
		if sstWriter.FileSize() >= sstFileSize && !bytes.Equal(lastKey, pair.Key) {
			if err := sstWriter.Finish(); err != nil {
				return err
			}
			if err := openNextFile(); err != nil {
				return err
			}
		}

		key := cloneAppend(prefix, pair.Key)
		if pair.Delete {
			err = sstWriter.DeleteWithTS(key, ts)
		} else {
			err = sstWriter.PutWithTS(key, ts, pair.Value)
		}
		if err != nil {
			return err
		}

		lastKey = pair.Key
	}
	return sstWriter.Finish()
}

// scanChangeSetFiles find change set files from the directory and sort them by the first version included, filter out
// empty files.
func scanChangeSetFiles(changeSetDir, store string) ([]FileWithVersion, error) {
	// scan directory to find the change set files
	storeDir := filepath.Join(changeSetDir, store)
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		// assume the change set files are taken from older versions, don't include all stores.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	fileNames := make([]string, len(entries))
	for i, entry := range entries {
		fileNames[i] = filepath.Join(storeDir, entry.Name())
	}
	return SortFilesByFirstVerson(fileNames)
}

// sstFileName inserts the seq integer into the base file name
func sstFileName(store string, seq int) string {
	return fmt.Sprintf("%s-%d%s", store, seq, SSTFileExtension)
}

func newSSTFileWriter() *grocksdb.SSTFileWriter {
	envOpts := grocksdb.NewDefaultEnvOptions()
	return grocksdb.NewSSTFileWriter(envOpts, tsrocksdb.NewVersionDBOpts(true))
}

// encodeSorterItem encode kv-pair for use in external sorter.
//
// layout: key + version(8) + delete(1) + [ value ] + key length(SizeKeyLength)
// we put the key and version in the front of payload so it can take advantage of the delta encoding in the `ExtSorter`.
func encodeSorterItem(version uint64, pair *iavl.KVPair) []byte {
	item := make([]byte, sizeOfSorterItem(pair))
	copy(item, pair.Key)
	offset := len(pair.Key)

	binary.LittleEndian.PutUint64(item[offset:], version)
	offset += tsrocksdb.TimestampSize

	if pair.Delete {
		item[offset] = 1
		offset++
	} else {
		copy(item[offset+1:], pair.Value)
		offset += len(pair.Value) + 1
	}
	binary.LittleEndian.PutUint32(item[offset:], uint32(len(pair.Key)))
	return item
}

// sizeOfSorterItem compute the encoded size of pair
//
// see godoc of `encodeSorterItem` for layout
func sizeOfSorterItem(pair *iavl.KVPair) int {
	size := len(pair.Key) + tsrocksdb.TimestampSize + 1 + SizeKeyLength
	if !pair.Delete {
		size += len(pair.Value)
	}
	return size
}

// decodeSorterItem decode the kv-pair from external sorter.
//
// see godoc of `encodeSorterItem` for layout
func decodeSorterItem(item []byte) ([]byte, iavl.KVPair) {
	var value []byte
	keyLen := binary.LittleEndian.Uint32(item[len(item)-SizeKeyLength:])
	key := item[:keyLen]

	offset := keyLen
	version := item[offset : offset+tsrocksdb.TimestampSize]

	offset += tsrocksdb.TimestampSize
	delete := item[offset] == 1

	if !delete {
		offset++
		value = item[offset : len(item)-SizeKeyLength]
	}

	return version, iavl.KVPair{
		Delete: delete,
		Key:    key,
		Value:  value,
	}
}

// compareSorterItem compare encoded kv-pairs return if a < b.
func compareSorterItem(a, b []byte) bool {
	// decode key and version
	aKeyLen := binary.LittleEndian.Uint32(a[len(a)-SizeKeyLength:])
	bKeyLen := binary.LittleEndian.Uint32(b[len(b)-SizeKeyLength:])
	ret := bytes.Compare(a[:aKeyLen], b[:bKeyLen])
	if ret != 0 {
		return ret == -1
	}

	aVersion := binary.LittleEndian.Uint64(a[aKeyLen:])
	bVersion := binary.LittleEndian.Uint64(b[bKeyLen:])
	// Compare version.
	// For the same user key with different timestamps, larger (newer) timestamp
	// comes first.
	return aVersion > bVersion
}

func cloneAppend(bz []byte, tail []byte) (res []byte) {
	res = make([]byte, len(bz)+len(tail))
	copy(res, bz)
	copy(res[len(bz):], tail)
	return
}
