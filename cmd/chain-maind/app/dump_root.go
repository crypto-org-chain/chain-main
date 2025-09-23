package app

import (
	"bytes"
	"fmt"
	"sort"

	dbm "github.com/cosmos/cosmos-db"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	"github.com/crypto-org-chain/chain-main/v4/app"
	chainmaintypes "github.com/crypto-org-chain/chain-main/v4/x/chainmain/types"
	nfttransfertypes "github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
	nfttypes "github.com/crypto-org-chain/chain-main/v4/x/nft/types"
	supplytypes "github.com/crypto-org-chain/chain-main/v4/x/supply/types"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	"cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/group"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
		DumpIavlRoot(storeNames),
	)
	return cmd
}

func DumpIavlRoot(storeNames []string) *cobra.Command {
	// this needs to change in different height
	// because some StoreKey version are zero
	// such as consensusparamtypes, circuittypes
	storeNames = []string{
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		minttypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		paramstypes.StoreKey,
		ibcexported.StoreKey,
		upgradetypes.StoreKey,
		feegrant.StoreKey,
		evidencetypes.StoreKey,
		ibctransfertypes.StoreKey,
		icacontrollertypes.StoreKey,
		icahosttypes.StoreKey,
		capabilitytypes.StoreKey,
		authzkeeper.StoreKey,
		nfttransfertypes.StoreKey,
		group.StoreKey,
		chainmaintypes.StoreKey,
		supplytypes.StoreKey,
		// maxsupplytypes.StoreKey,
		nfttypes.StoreKey,
		// consensusparamtypes.StoreKey,
		// circuittypes.StoreKey,
		"icaauth",
	}

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
			db, err := dbm.NewGoLevelDB("application", dir, dbm.OptionsMap{"read_only": true})
			if err != nil {
				return err
			}
			defer db.Close()
			rs := rootmulti.NewStore(db, log.NewNopLogger(), nil)
			for _, storeKey := range storeNames {
				rs.MountStoreWithDB(types.NewKVStoreKey(storeKey), types.StoreTypeIAVL, nil)
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
				storeMaps := make(map[string]struct{})
				for _, storeName := range storeNames {
					storeMaps[storeName] = struct{}{}
				}
				for _, info := range cInfo.StoreInfos {
					if _, ok := storeMaps[info.Name]; !ok {
						fmt.Printf("module %s missed\n", info.Name)
					}
				}
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
