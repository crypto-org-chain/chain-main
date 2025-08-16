package client

import (
	"bufio"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	log "cosmossdk.io/log"
	"github.com/alitto/pond"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/iavl"
	"github.com/golang/snappy"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"cosmossdk.io/store/wrapper"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"

	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

const DefaultChunkSize = 1000000

func DumpChangeSetCmd(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump outDir",
		Short: "Extract changesets from iavl versions, and save to plain file format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			if err := ctx.Viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			db, err := opts.OpenReadOnlyDB(ctx.Viper.GetString(flags.FlagHome), server.GetAppDBBackend(ctx.Viper))
			if err != nil {
				return err
			}

			cacheSize := cast.ToInt(ctx.Viper.Get(server.FlagIAVLCacheSize))

			startVersion, err := cmd.Flags().GetInt64(flagStartVersion)
			if err != nil {
				return err
			}
			endVersion, err := cmd.Flags().GetInt64(flagEndVersion)
			if err != nil {
				return err
			}
			concurrency, err := cmd.Flags().GetInt(flagConcurrency)
			if err != nil {
				return err
			}
			chunkSize, err := cmd.Flags().GetInt(flagChunkSize)
			if err != nil {
				return err
			}
			zlibLevel, err := cmd.Flags().GetInt(flagZlibLevel)
			if err != nil {
				return err
			}
			stores, err := GetStoresOrDefault(cmd, opts.DefaultStores)
			if err != nil {
				return err
			}
			outDir := args[0]
			if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
				return err
			}
			iavlVersion, err := cmd.Flags().GetInt(flagIAVLVersion)
			if err != nil {
				return err
			}

			if endVersion == 0 {
				// use the latest version of the first store for all stores
				prefix := []byte(fmt.Sprintf(tsrocksdb.StorePrefixTpl, stores[0]))
				tree := iavl.NewMutableTree(wrapper.NewDBWrapper((dbm.NewPrefixDB(db, prefix))), 0, true, log.NewNopLogger())
				latestVersion, err := tree.LoadVersion(0)
				if err != nil {
					return err
				}
				endVersion = latestVersion + 1
				fmt.Println("end version not specified, default to latest version + 1,", endVersion)
			}

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 1024)
			defer pool.StopAndWait()

			// we handle multiple stores sequentially, because different stores don't share much in db, handle concurrently reduces cache efficiency.
			for _, store := range stores {
				fmt.Println("begin store", store, time.Now().Format(time.RFC3339))

				// find the first version in the db, reading raw db because no public api for it.
				prefix := []byte(fmt.Sprintf(tsrocksdb.StorePrefixTpl, store))
				storeStartVersion, err := getFirstVersion(dbm.NewPrefixDB(db, prefix), iavlVersion)
				if err != nil {
					return err
				}
				if storeStartVersion == 0 {
					// store not exists
					fmt.Println("skip empty store")
					continue
				}
				if startVersion > storeStartVersion {
					storeStartVersion = startVersion
				}

				// share the iavl tree between tasks to reuse the node cache
				iavlTreePool := sync.Pool{
					New: func() any {
						// use separate prefixdb and iavl tree in each task to maximize concurrency performance
						return iavl.NewImmutableTree(wrapper.NewDBWrapper(dbm.NewPrefixDB(db, prefix)), cacheSize, true, log.NewNopLogger())
					},
				}

				// first split work load into chunks
				var chunks []chunk
				for i := storeStartVersion; i < endVersion; i += int64(chunkSize) {
					end := i + int64(chunkSize)
					if end > endVersion {
						end = endVersion
					}

					var taskFiles []string
					group, _ := pool.GroupContext(context.Background())
					// then split each chunk according to number of workers, the results will be concatenated into a single chunk file
					for _, workRange := range splitWorkLoad(concurrency, Range{Start: i, End: end}) {
						// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
						workRange := workRange
						taskFile := filepath.Join(outDir, fmt.Sprintf("tmp-%s-%d.snappy", store, workRange.Start))
						group.Submit(func() error {
							tree := iavlTreePool.Get().(*iavl.ImmutableTree)
							defer iavlTreePool.Put(tree)
							return dumpRangeBlocks(taskFile, tree, workRange)
						})

						taskFiles = append(taskFiles, taskFile)
					}

					chunks = append(chunks, chunk{
						store: store, beginVersion: i, taskFiles: taskFiles, taskGroup: group,
					})
				}

				// for each chunk, wait for related tasks to finish, and concatenate the result files in order
				for _, chunk := range chunks {
					if err := chunk.collect(outDir, zlibLevel); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().Int64(flagStartVersion, 0, "The start version")
	cmd.Flags().Int64(flagEndVersion, 0, "The end version, exclusive, default to latestVersion+1")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")
	cmd.Flags().Int(server.FlagIAVLCacheSize, 781250, "size of the iavl tree cache")
	cmd.Flags().Int(flagChunkSize, DefaultChunkSize, "size of the block chunk")
	cmd.Flags().Int(flagZlibLevel, 6, "level of zlib compression, 0: plain data, 1: fast, 9: best, default: 6, if not 0 the output file name will have .zz extension")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().Int(flagIAVLVersion, IAVLV1, "IAVL version, 0: v0, 1: v1")
	return cmd
}

// Range represents a range `[start, end)`
type Range struct {
	Start, End int64
}

func splitWorkLoad(workers int, full Range) []Range {
	var chunks []Range
	chunkSize := (full.End - full.Start + int64(workers) - 1) / int64(workers)
	for i := full.Start; i < full.End; i += chunkSize {
		end := i + chunkSize
		if end > full.End {
			end = full.End
		}
		chunks = append(chunks, Range{Start: i, End: end})
	}
	return chunks
}

func dumpRangeBlocks(outputFile string, tree *iavl.ImmutableTree, blockRange Range) (returnErr error) {
	fp, err := createFile(outputFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := fp.Close(); returnErr == nil {
			returnErr = err
		}
	}()

	writer := snappy.NewBufferedWriter(fp)

	// TraverseStateChanges becomes inclusive on end since iavl `v1.x.x`, while the blockRange is exclusive on end
	if err := tree.TraverseStateChanges(blockRange.Start, blockRange.End-1, func(version int64, changeSet *iavl.ChangeSet) error {
		return WriteChangeSet(writer, version, changeSet)
	}); err != nil {
		return err
	}

	return writer.Flush()
}

type chunk struct {
	store        string
	beginVersion int64
	taskFiles    []string
	taskGroup    *pond.TaskGroupWithContext
}

// collect wait for the tasks to complete and concatenate the files into a single output file.
func (c *chunk) collect(outDir string, zlibLevel int) (returnErr error) {
	storeDir := filepath.Join(outDir, c.store)
	if err := os.MkdirAll(storeDir, os.ModePerm); err != nil {
		return err
	}

	output := filepath.Join(storeDir, fmt.Sprintf("block-%d", c.beginVersion))
	if zlibLevel > 0 {
		output += ZlibFileSuffix
	}

	if err := c.taskGroup.Wait(); err != nil {
		return err
	}

	fp, err := createFile(output)
	if err != nil {
		return err
	}
	defer func() {
		if err := fp.Close(); returnErr == nil {
			returnErr = err
		}
	}()

	bufWriter := bufio.NewWriter(fp)
	writer := io.Writer(bufWriter)

	var zwriter *zlib.Writer
	if zlibLevel > 0 {
		var err error
		zwriter, err = zlib.NewWriterLevel(bufWriter, zlibLevel)
		if err != nil {
			return err
		}

		writer = zwriter
	}

	for _, taskFile := range c.taskFiles {
		if err := copyTmpFile(writer, taskFile); err != nil {
			return err
		}
		if err := os.Remove(taskFile); err != nil {
			return err
		}
	}

	if zwriter != nil {
		if err := zwriter.Close(); err != nil {
			return err
		}
	}

	return bufWriter.Flush()
}

// copyTmpFile append the snappy compressed temporary file to writer
func copyTmpFile(writer io.Writer, tmpFile string) error {
	fp, err := os.Open(tmpFile)
	if err != nil {
		return err
	}
	defer fp.Close()

	_, err = io.Copy(writer, snappy.NewReader(fp))
	return err
}

func createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}

func getFirstVersion(db dbm.DB, iavlVersion int) (int64, error) {
	if iavlVersion == IAVLV0 {
		itr, err := db.Iterator(
			rootKeyFormat.Key(uint64(1)),
			rootKeyFormat.Key(uint64(math.MaxInt64)),
		)
		if err != nil {
			return 0, err
		}
		defer itr.Close()

		var version int64
		for ; itr.Valid(); itr.Next() {
			rootKeyFormat.Scan(itr.Key(), &version)
			return version, nil
		}

		return 0, itr.Error()
	}

	itr, err := db.Iterator(
		nodeKeyV1Format.KeyInt64(1),
		nodeKeyV1Format.KeyInt64(math.MaxInt64),
	)
	if err != nil {
		return 0, err
	}
	defer itr.Close()

	for ; itr.Valid(); itr.Next() {
		var nk []byte
		nodeKeyV1Format.Scan(itr.Key(), &nk)
		version := int64(binary.BigEndian.Uint64(nk))
		nonce := binary.BigEndian.Uint32(nk[8:])
		if nonce == 1 {
			// root key is normal node key with nonce 1
			return version, nil
		}
	}

	return 0, itr.Error()
}
