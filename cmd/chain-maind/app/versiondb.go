//go:build rocksdb
// +build rocksdb

package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/chain-main/v4/cmd/chain-maind/opendb"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/linxGnu/grocksdb"
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

func GetChangeSetCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)

	return GetChangeSetGroupCmd(versiondbclient.Options{
		DefaultStores:     storeNames,
		OpenReadOnlyDB:    opendb.OpenReadOnlyDB,
		AppRocksDBOptions: opendb.NewRocksdbOptions,
	})
}

func GetChangeSetGroupCmd(opts versiondbclient.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "getversionchangeset",
		Short: "dump and manage change sets files and ingest into versiondb",
	}
	cmd.AddCommand(
		getVersionDBAtVersionCmd(opts),
		getIAVLAtVersionCmd(opts),
	)
	return cmd
}

func getVersionDBAtVersionCmd(opts versiondbclient.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versiondb-at-version",
		Short: "versiondb at version [dir] [outDir]",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			dir := args[0]
			outDir := args[1]

			version, err := cmd.Flags().GetInt64(flagVersion)
			if err != nil {
				return err
			}
			var (
				db       *grocksdb.DB
				cfHandle *grocksdb.ColumnFamilyHandle
			)

			//db, cfHandle, err = tsrocksdb.OpenVersionDBForReadOnly(dir, false)
			db, cfHandle, err = tsrocksdb.OpenVersionDB(dir)
			if err != nil {
				return err
			}
			defer db.Close()
			versionDB := tsrocksdb.NewStoreWithDB(db, cfHandle)
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
				for ; it.Valid(); it.Next() {
					buf := fmt.Sprintf("key = %v, value = %v\n", it.Key(), it.Value())
					_, err = kvsWriter.WriteString(buf)
					if err != nil {
						return err
					}
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
	cmd.Flags().Int64(flagVersion, 0, "the version to print")
	return cmd
}

func getIAVLAtVersionCmd(opts versiondbclient.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iavl-at-version",
		Short: "iavl at version [dir] [outDir]",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, err := cmd.Flags().GetInt64(flagVersion)
			if err != nil {
				return err
			}
			fmt.Printf("todo implement %s %s %d\n", args[0], args[1], version)
			return nil
		},
	}
	cmd.Flags().Int64(flagVersion, 0, "the version to print")
	return cmd
}

func createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}
