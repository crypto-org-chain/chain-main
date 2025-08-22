package app

import (
	"fmt"
	"os"
	"sort"

	"cosmossdk.io/log"
	"cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/spf13/cobra"

	"github.com/cosmos/iavl"
	idbm "github.com/cosmos/iavl/db"
)

func DumpRootCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)
	return DumpRootGroupCmd(storeNames)
}

func DumpRootGroupCmd(storeNames []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-root",
		Short: "dump module root",
	}
	cmd.AddCommand(
		DumpMemIavlRoot(storeNames),
		DumpIavlRoot(storeNames),
	)
	return cmd
}

func DumpMemIavlRoot(storeNames []string) *cobra.Command {
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
					fmt.Printf("module %s not load\n", storeName)
				}
			}

			db.MultiTree.UpdateCommitInfo()
			lastCommitInfo := convertCommitInfo(db.MultiTree.LastCommitInfo())

			fmt.Printf("Version %d RootHash %X\n", lastCommitInfo.Version, lastCommitInfo.Hash())
			return nil
		},
	}
	cmd.Flags().Uint32("version", 0, "the version to dump")
	return cmd
}

type storeParams struct {
	key types.StoreKey
	typ types.StoreType
}

func mergeStoreInfos(commitInfo *types.CommitInfo, storeInfos []types.StoreInfo) *types.CommitInfo {
	infos := make([]types.StoreInfo, 0, len(commitInfo.StoreInfos)+len(storeInfos))
	infos = append(infos, commitInfo.StoreInfos...)
	infos = append(infos, storeInfos...)
	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return &types.CommitInfo{
		Version:    commitInfo.Version,
		StoreInfos: infos,
	}
}

// amendCommitInfo add mem stores commit infos to keep it compatible with cosmos-sdk 0.46
func amendCommitInfo(commitInfo *types.CommitInfo, storeParams map[types.StoreKey]storeParams) *types.CommitInfo {
	var extraStoreInfos []types.StoreInfo
	for key := range storeParams {
		typ := storeParams[key].typ
		if typ != types.StoreTypeIAVL && typ != types.StoreTypeTransient {
			extraStoreInfos = append(extraStoreInfos, types.StoreInfo{
				Name:     key.Name(),
				CommitId: types.CommitID{},
			})
		}
	}
	return mergeStoreInfos(commitInfo, extraStoreInfos)
}

func convertCommitInfo(commitInfo *memiavl.CommitInfo) *types.CommitInfo {
	storeInfos := make([]types.StoreInfo, len(commitInfo.StoreInfos))
	for i, storeInfo := range commitInfo.StoreInfos {
		storeInfos[i] = types.StoreInfo{
			Name: storeInfo.Name,
			CommitId: types.CommitID{
				Version: storeInfo.CommitId.Version,
				Hash:    storeInfo.CommitId.Hash,
			},
		}
	}
	return &types.CommitInfo{
		Version:    commitInfo.Version,
		StoreInfos: storeInfos,
	}
}

func DumpIavlRoot(storeNames []string) *cobra.Command {
	storeNames = []string{"acc", "authz", "bank", "capability", "chainmain", "distribution", "evidence", "feegrant",
		"feeibc", "gov", "group", "ibc", "icacontroller", "icahost", "mint", "nft", "nonfungibletokentransfer",
		"params", "slashing", "staking", "supply", "transfer", "upgrade"}
	cmd := &cobra.Command{
		Use:   "dump-iavl-root",
		Short: "dump iavl root at version [dir]",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			version, err := cmd.Flags().GetInt64("version")
			if err != nil {
				return err
			}
			db, err := dbm.NewGoLevelDB("application", dir, nil)
			if err != nil {
				return err
			}
			defer db.Close()
			tree := iavl.NewMutableTree(idbm.NewWrapper(db), 10000, false, log.NewLogger(os.Stdout))
			ver, err := tree.LoadVersion(version)
			fmt.Printf("Version %d RootHash %X\n", ver, tree.Hash())
			return nil
		},
	}
	cmd.Flags().Int64("version", 0, "the version to dump")
	return cmd
}
