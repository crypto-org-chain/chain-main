package client

import (
	"github.com/cosmos/iavl"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/spf13/cobra"
)

func ChangeSetToVersionDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "to-versiondb versiondb-path plain-1 [plain-2] ...",
		Short: "Feed change set files into versiondb",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cmd.Flags().GetString(flagStore)
			if err != nil {
				return err
			}

			versionDB, err := tsrocksdb.NewStore(args[0])
			if err != nil {
				return err
			}

			for _, plainFile := range args[1:] {
				if err := withChangeSetFile(plainFile, func(reader Reader) error {
					_, err := IterateChangeSets(reader, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
						if err := versionDB.FeedChangeSet(version, store, changeSet); err != nil {
							return false, err
						}
						return true, nil
					})

					return err
				}); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().String(flagStore, "", "store name, the keys are prefixed with \"s/k:{store}/\"")
	return cmd
}
