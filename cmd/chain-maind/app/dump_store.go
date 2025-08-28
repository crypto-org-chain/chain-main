package app

import (
	"fmt"
	"sort"
	"strings"

	"cosmossdk.io/log"
	"cosmossdk.io/store/types"

	"cosmossdk.io/store/rootmulti"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/spf13/cobra"
)

const capaMemStoreKey = "mem_capability"

func DumpStoreCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)
	return DumpStoreGroupCmd(storeNames)
}

func DumpStoreGroupCmd(storeNames []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-store",
		Short: "dump module store",
	}
	cmd.AddCommand(
		DumpMemIavlStore(storeNames),
		DumpIavlStore(storeNames),
	)
	return cmd
}

func DumpMemIavlStore(storeNames []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-memiavl-root",
		Short: "dump mem-iavl root at version [dir]",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			version, err := cmd.Flags().GetUint32("version")
			if err != nil {
				return err
			}
			opts := memiavl.Options{
				InitialStores:   storeNames,
				CreateIfMissing: false,
				TargetVersion:   version,
			}
			db, err := memiavl.Load(dir, opts)
			if err != nil {
				return err
			}
			for _, storeName := range storeNames {
				tree := db.TreeByName(storeName)
				if tree != nil {
					fmt.Printf("module %s version %d RootHash %X\n", storeName, tree.Version(), tree.RootHash())
				} else {
					fmt.Printf("module %s not loaded\n", storeName)
				}
			}

			db.MultiTree.UpdateCommitInfo()
			lastCommitInfo := convertCommitInfo(db.MultiTree.LastCommitInfo())

			fmt.Printf("Version %d RootHash %X\n", lastCommitInfo.Version, lastCommitInfo.Hash())

			tree := db.TreeByName(capaMemStoreKey)
			if tree != nil {
				fmt.Printf("module %s Version %d RootHash %X\n", capaMemStoreKey, tree.Version(), tree.Version())
			} else {
				fmt.Printf("module %s not loaded\n", capaMemStoreKey)
			}
			return nil
		},
	}
	cmd.Flags().Uint32("version", 0, "the version to dump")
	return cmd
}

func DumpIavlStore(storeNames []string) *cobra.Command {
	storeNames = []string{
		"acc", "authz", "bank", "capability", "chainmain", "distribution", "evidence", "feegrant",
		"feeibc", "gov", "group", "ibc", "icaauth", "icacontroller", "icahost", "mint", "nft", "nonfungibletokentransfer",
		"params", "slashing", "staking", "supply", "transfer", "upgrade",
	}

	short := fmt.Sprintf("dump iavl store at version [dir] [storeKey], use these storeKey %s", strings.Join(storeNames, ","))

	cmd := &cobra.Command{
		Use:   "dump-iavl-store",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			storeKey := args[1]
			version, err := cmd.Flags().GetInt64("version")
			if err != nil {
				return err
			}
			db, err := dbm.NewGoLevelDB("application", dir, nil)
			if err != nil {
				return err
			}
			defer db.Close()
			rs := rootmulti.NewStore(db, log.NewNopLogger(), nil)
			for _, storeKey := range storeNames {
				rs.MountStoreWithDB(types.NewKVStoreKey(storeKey), types.StoreTypeIAVL, nil)
			}

			err = rs.LoadLatestVersion()
			if err != nil {
				fmt.Printf("failed to load latest version: %s\n", err.Error())
				return err
			}
			err = rs.LoadVersion(version)
			if err != nil {
				fmt.Printf("failed to load  version %d %s\n", version, err.Error())
				return err
			}
			kvStore := rs.GetKVStore(types.NewKVStoreKey(storeKey))
			it := kvStore.Iterator(nil, nil)
			defer it.Close()

			fmt.Printf("version %d RootHash %X\n", version, it.Key())
			for ; it.Valid(); it.Next() {
				key := it.Key()
				value := it.Value()
				fmt.Printf("%s: %s\n", string(key), string(value))
			}
			return nil
		},
	}
	cmd.Flags().Int64("version", 0, "the version to dump")
	return cmd
}
