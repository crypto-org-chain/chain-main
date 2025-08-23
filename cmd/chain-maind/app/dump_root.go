package app

import (
	"bytes"
	"fmt"
	"sort"

	"cosmossdk.io/log"
	"cosmossdk.io/store/types"

	"cosmossdk.io/store/rootmulti"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/chain-main/v4/app"
	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/spf13/cobra"
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
					fmt.Printf("module %s not loaded\n", storeName)
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
		"feeibc", "gov", "group", "ibc", "icaauth", "icacontroller", "icahost", "mint", "nft", "nonfungibletokentransfer",
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

			var cInfo *types.CommitInfo
			cInfo, err = rs.GetCommitInfo(version)
			if err != nil {
				fmt.Printf("failed to load version %d commit info: %s\n", version, err.Error())
				return err
			}
			infoMaps := make(map[string]types.StoreInfo)
			for _, storeInfo := range cInfo.StoreInfos {
				infoMaps[storeInfo.Name] = storeInfo
			}

			var infos []types.StoreInfo
			for _, storeName := range storeNames {
				info, ok := infoMaps[storeName]
				if !ok {
					fmt.Printf("module %s not loaded\n", storeName)
					continue
				}
				commitID := info.CommitId
				fmt.Printf("module %s version %d RootHash %X\n", storeName, commitID.Version, commitID.Hash)
				infos = append(infos, info)
			}

			if len(infos) != len(cInfo.StoreInfos) {
				fmt.Printf("Warning: Partial commit info (loaded %d stores, found %d)\n", len(cInfo.StoreInfos), len(infos))
			}

			commitInfo := &types.CommitInfo{
				Version:    version,
				StoreInfos: infos,
			}

			if rs.LastCommitID().Version != commitInfo.Version || !bytes.Equal(rs.LastCommitID().Hash, commitInfo.Hash()) {
				return fmt.Errorf("failed to calculate %d commit info, rs Hash %X, commit Hash %X", rs.LastCommitID().Version, rs.LastCommitID().Hash, commitInfo.Hash())
			}
			fmt.Printf("Version %d RootHash %X\n", commitInfo.Version, commitInfo.Hash())
			return nil
		},
	}
	cmd.Flags().Int64("version", 0, "the version to dump")
	return cmd
}
