//go:build rocksdb
// +build rocksdb

package app

import (
	"bufio"
	"cosmossdk.io/store/types"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cosmos/iavl"
	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/chain-main/v4/cmd/chain-maind/opendb"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/spf13/cobra"
)

const (
	flagVersion = "version"
)

func ChangeSetCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)

	return versiondbclient.ChangeSetGroupCmd(versiondbclient.Options{
		DefaultStores:     storeNames,
		OpenReadOnlyDB:    opendb.OpenReadOnlyDB,
		AppRocksDBOptions: opendb.NewRocksdbOptions,
	})
}

func VersionDBChangeSetCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)

	return DumpVersionDBChangeSet(versiondbclient.Options{
		DefaultStores:     storeNames,
		OpenReadOnlyDB:    opendb.OpenReadOnlyDB,
		AppRocksDBOptions: opendb.NewRocksdbOptions,
	})
}

func DumpVersionDBChangeSet(opts versiondbclient.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-versiondb",
		Short: "dump versiondb changeset at version [dir] [outDir]",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			outDir := args[1]

			version, err := cmd.Flags().GetInt64(flagVersion)
			if err != nil {
				return err
			}
			versionDB, err := tsrocksdb.NewStore(dir)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
				return err
			}

			for _, storeKey := range opts.DefaultStores {
				it, err := versionDB.IteratorAtVersion(storeKey, nil, nil, &version)
				if err != nil {
					return err
				}
				defer it.Close()

				kvsFile := filepath.Join(outDir, storeKey)
				fpKvs, err := createFile(kvsFile)
				if err != nil {
					return err
				}
				kvsWriter := bufio.NewWriter(fpKvs)

				var pairs []*iavl.KVPair
				for ; it.Valid(); it.Next() {
					key := make([]byte, len(it.Key()))
					copy(key, it.Key())
					value := make([]byte, len(it.Value()))
					copy(value, it.Value())
					pair := &iavl.KVPair{Key: key, Value: value}
					if len(pair.Value) == 0 {
						pair.Delete = true
					}
					pairs = append(pairs, pair)
				}
				changeset := &iavl.ChangeSet{Pairs: pairs}
				// https://github.com/crypto-org-chain/cronos/blob/28bc916d0903050ac30ddd79805f451bc38923d3/versiondb/client/changeset.go#L43
				err = versiondbclient.WriteChangeSet(kvsWriter, version, changeset)
				if err != nil {
					return err
				}
				err = kvsWriter.Flush()
				if err != nil {
					return err
				}
				err = fpKvs.Close()
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().Int64(flagVersion, 0, "the version to dump")
	return cmd
}

func createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}

func FixVersionDBCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)
	return FixVersionDB(versiondbclient.Options{
		DefaultStores:     storeNames,
		OpenReadOnlyDB:    opendb.OpenReadOnlyDB,
		AppRocksDBOptions: opendb.NewRocksdbOptions,
	})
}

func FixVersionDB(opts versiondbclient.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-versiondb",
		Short: "fix versiondb changeset at version [dir]",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			version, err := cmd.Flags().GetInt64(flagVersion)
			if err != nil {
				return err
			}
			versionDB, err := tsrocksdb.NewStore(dir)
			if err != nil {
				return err
			}
			set := make(map[string]struct{})

			for _, storeKey := range opts.DefaultStores {
				set[storeKey] = struct{}{}
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				return err
			}

			versionDB, err := tsrocksdb.NewStore(dir)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				if _, ok := set[entry.Name()]; !ok {
					fmt.Errorf("illegal file %s\n", entry.Name())
					continue
				}
				it, err := versionDB.IteratorAtVersion(entry.Name(), nil, nil, &version)
				if err != nil {
					return err
				}
				var delSet []*types.StoreKVPair
				for ; it.Valid(); it.Next() {
					key := make([]byte, len(it.Key()))
					copy(key, it.Key())
					delSet = append(delSet, &types.StoreKVPair{Key: key, Delete: true, StoreKey: entry.Name()})
				}

				it.Close()
				versionDB.PutAtVersion(version, delSet)

				filePath := filepath.Join(dir, entry.Name())
				kvsFile, err := os.OpenFile(filePath, os.O_RDONLY, 0o600)
				if err != nil {
					fmt.Errorf("open illegal file %s\n %v", entry.Name(), err.Error())
					continue
				}
				kvsReader := bufio.NewReader(fpKvs)
				ver, count, addchangeset, err := versiondbclient.ReadChangeSet(kvsReader, true)
				if err != nil {
					fmt.Errorf("readchangeset illegal file %s\n %v", entry.Name(), err.Error())
					continue
				}
				kvsFile.Close()

				var addSet []*iavl.StoreKVPair
				addSet := make([]*iavl.StoreKVPair, 0, len(addchangeset))
				for _, kv := range addchangeset.Pairs {
					key := make([]byte, len(kv.Key()))
					copy(key, kv.Key())
					if kv.Delete {
						addSet = append(addSet, &iavl.StoreKVPair{Key: key, Delete: false})
						continue
					}
					value := make([]byte, len(kv.Value()))
					copy(value, kv.Value())
					addSet = append(addSet, &iavl.StoreKVPair{Key: key, Value: value})
				}

				versionDB.PutAtVersion(version, addSet)
			}
			return nil
		},
	}
	cmd.Flags().Int64(flagVersion, 0, "the version to dump")
	return cmd
}
