package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"github.com/alitto/pond"
	"github.com/cosmos/gogoproto/jsonpb"
	"github.com/cosmos/iavl"
	"github.com/spf13/cobra"

	storetypes "cosmossdk.io/store/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"

	"github.com/crypto-org-chain/cronos/memiavl"
)

func VerifyChangeSetCmd(defaultStores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify changeSetDir",
		Short: "Replay the input change set files in order to rebuild iavl tree in memory and output app hash and full json encoded commit info, user can compare the root hash against the block headers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			concurrency, err := cmd.Flags().GetInt(flagConcurrency)
			if err != nil {
				return err
			}
			targetVersion, err := cmd.Flags().GetInt64(flagTargetVersion)
			if err != nil {
				return err
			}
			saveSnapshot, err := cmd.Flags().GetString(flagSaveSnapshot)
			if err != nil {
				return err
			}
			loadSnapshot, err := cmd.Flags().GetString(flagLoadSnapshot)
			if err != nil {
				return err
			}
			check, err := cmd.Flags().GetBool(flagCheck)
			if err != nil {
				return err
			}
			save, err := cmd.Flags().GetBool(flagSave)
			if err != nil {
				return err
			}
			stores, err := GetStoresOrDefault(cmd, defaultStores)
			if err != nil {
				return err
			}

			if len(saveSnapshot) > 0 {
				// detect the write permission early on.
				if err := os.MkdirAll(saveSnapshot, os.ModePerm); err != nil {
					return err
				}
			}

			changeSetDir := args[0]

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 0)
			defer pool.StopAndWait()
			group, _ := pool.GroupContext(context.Background())

			var (
				lastestVersion int64
				storeInfosLock sync.Mutex
			)
			storeInfos := []storetypes.StoreInfo{
				// https://github.com/cosmos/cosmos-sdk/issues/14916
				{Name: capabilitytypes.MemStoreKey, CommitId: storetypes.CommitID{}},
			}

			mtree := memiavl.NewEmptyMultiTree(0, 0)
			if len(loadSnapshot) > 0 {
				var err error
				mtree, err = memiavl.LoadMultiTree(loadSnapshot, true, 0)
				if err != nil {
					return err
				}
			}

			for _, store := range stores {
				// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
				store := store
				tree := mtree.TreeByName(store)
				if tree == nil {
					tree = memiavl.New(0)
				}
				group.Submit(func() error {
					storeInfo, err := verifyOneStore(tree, store, changeSetDir, saveSnapshot, targetVersion)
					if err != nil {
						return err
					}
					if storeInfo == nil {
						// the store don't exist before target version, don't affect the commit info and app hash.
						return nil
					}

					storeInfosLock.Lock()
					defer storeInfosLock.Unlock()
					storeInfos = append(storeInfos, *storeInfo)
					if storeInfo.CommitId.Version > lastestVersion {
						lastestVersion = storeInfo.CommitId.Version
					}
					return nil
				})
			}
			if err := group.Wait(); err != nil {
				return err
			}

			commitInfo := buildCommitInfo(storeInfos, lastestVersion)

			if len(saveSnapshot) > 0 {
				// write multitree metadata
				metadata := memiavl.MultiTreeMetadata{
					CommitInfo: convertCommitInfo(&commitInfo),
				}
				bz, err := metadata.Marshal()
				if err != nil {
					return err
				}
				if err := memiavl.WriteFileSync(filepath.Join(saveSnapshot, memiavl.MetadataFileName), bz); err != nil {
					return err
				}
			}

			// write out the replay result
			var buf bytes.Buffer
			buf.WriteString(hex.EncodeToString(commitInfo.Hash()))
			buf.WriteString("\n")
			marshaler := jsonpb.Marshaler{}
			if err := marshaler.Marshal(&buf, &commitInfo); err != nil {
				return err
			}

			verifiedFileName := filepath.Join(changeSetDir, fmt.Sprintf("verified-%d", commitInfo.Version))
			if check {
				// check commitInfo against the one stored in change set
				bz, err := os.ReadFile(verifiedFileName)
				if err != nil {
					return err
				}

				if !bytes.Equal(buf.Bytes(), bz) {
					return fmt.Errorf("verify result don't match")
				}

				fmt.Printf("version %d checked successfully\n", commitInfo.Version)
				return nil
			}

			if save {
				if err := os.WriteFile(verifiedFileName, buf.Bytes(), 0o600); err != nil {
					return err
				}
				fmt.Printf("version %d verify result saved to %s\n", commitInfo.Version, verifiedFileName)
				return nil
			}

			_, err = os.Stdout.Write(buf.Bytes())
			return err
		},
	}

	cmd.Flags().Int64(flagTargetVersion, 0, "specify the target version, otherwise it'll exhaust the plain files")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().String(flagSaveSnapshot, "", "save the snapshot of the target iavl tree to directory")
	cmd.Flags().String(flagLoadSnapshot, "", "load the snapshot before doing verification from directory")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")
	cmd.Flags().Bool(flagCheck, false, "Check the replayed hash with the one stored in change set directory")
	cmd.Flags().Bool(flagSave, false, "Save the verify result to change set directory, otherwise output to stdout")

	return cmd
}

// verifyOneStore process a single store, can run in parallel with other stores.
// if the store don't exist before the `targetVersion`, returns nil without error.
func verifyOneStore(tree *memiavl.Tree, store, changeSetDir, saveSnapshot string, targetVersion int64) (*storetypes.StoreInfo, error) {
	filesWithVersion, err := scanChangeSetFiles(changeSetDir, store)
	if err != nil {
		return nil, err
	}

	if len(filesWithVersion) == 0 {
		return nil, nil
	}
	// set the initial version for the store
	initialVersion := filesWithVersion[0].Version
	if targetVersion > 0 && initialVersion > uint64(targetVersion) {
		return nil, nil
	}

	if err := tree.SetInitialVersion(int64(initialVersion)); err != nil {
		return nil, err
	}

	for _, file := range filesWithVersion {
		if targetVersion > 0 && file.Version > uint64(targetVersion) {
			break
		}

		err = withChangeSetFile(file.FileName, func(reader Reader) error {
			_, err := IterateChangeSets(reader, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
				if version <= tree.Version() {
					// skip old change sets
					return true, nil
				}

				// no need to update hashes for intermediate versions.
				tree.ApplyChangeSet(convertChangeSet(changeSet))
				_, v, err := tree.SaveVersion(false)
				if err != nil {
					return false, err
				}
				if v != version {
					return false, fmt.Errorf("version don't match: %d != %d", v, version)
				}
				return targetVersion == 0 || v < targetVersion, nil
			})

			return err
		})
		if err != nil {
			break
		}

		if targetVersion > 0 && tree.Version() >= targetVersion {
			break
		}
	}

	if err != nil {
		return nil, err
	}

	if len(saveSnapshot) > 0 {
		snapshotDir := filepath.Join(saveSnapshot, store)
		if err := os.MkdirAll(snapshotDir, os.ModePerm); err != nil {
			return nil, err
		}
		if err := tree.WriteSnapshot(snapshotDir); err != nil {
			return nil, err
		}
	}

	return &storetypes.StoreInfo{
		Name:     store,
		CommitId: lastCommitID(tree),
	}, nil
}

// lastCommitID build `CommitID` from a memiavl tree.
func lastCommitID(tree *memiavl.Tree) storetypes.CommitID {
	// copy out the hash in case it's relied on mmap-ed file.
	var hash [memiavl.SizeHash]byte
	copy(hash[:], tree.RootHash())
	return storetypes.CommitID{
		Version: tree.Version(),
		Hash:    hash[:],
	}
}

// buildCommitInfo sort the storeInfos by store name, and built `CommitInfo`.
func buildCommitInfo(storeInfos []storetypes.StoreInfo, version int64) storetypes.CommitInfo {
	sort.SliceStable(storeInfos, func(i, j int) bool {
		return storeInfos[i].Name < storeInfos[j].Name
	})

	return storetypes.CommitInfo{
		Version:    storeInfos[0].CommitId.Version,
		StoreInfos: storeInfos,
	}
}

func convertCommitInfo(commitInfo *storetypes.CommitInfo) *memiavl.CommitInfo {
	storeInfos := make([]memiavl.StoreInfo, len(commitInfo.StoreInfos))
	for i, storeInfo := range commitInfo.StoreInfos {
		storeInfos[i] = memiavl.StoreInfo{
			Name: storeInfo.Name,
			CommitId: memiavl.CommitID{
				Version: storeInfo.CommitId.Version,
				Hash:    storeInfo.CommitId.Hash,
			},
		}
	}
	return &memiavl.CommitInfo{
		Version:    commitInfo.Version,
		StoreInfos: storeInfos,
	}
}

func convertChangeSet(cs *iavl.ChangeSet) memiavl.ChangeSet {
	pairs := make([]*memiavl.KVPair, len(cs.Pairs))
	for i, pair := range cs.Pairs {
		pairs[i] = &memiavl.KVPair{
			Delete: pair.Delete,
			Key:    pair.Key,
			Value:  pair.Value,
		}
	}
	return memiavl.ChangeSet{
		Pairs: pairs,
	}
}
