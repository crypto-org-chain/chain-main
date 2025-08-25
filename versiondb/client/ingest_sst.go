package client

import (
	"fmt"

	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

func IngestVersionDBSSTCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest-versiondb-sst versiondb-path [file1.sst file2.sst ...]",
		Short: "Ingest sst files into versiondb and update the latest version",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := args[0]
			moveFiles, err := cmd.Flags().GetBool(flagMoveFiles)
			if err != nil {
				return err
			}

			db, cfHandle, err := tsrocksdb.OpenVersionDB(dbPath)
			if err != nil {
				return err
			}
			if len(args) > 1 {
				ingestOpts := grocksdb.NewDefaultIngestExternalFileOptions()
				ingestOpts.SetMoveFiles(moveFiles)
				if err := db.IngestExternalFileCF(cfHandle, args[1:], ingestOpts); err != nil {
					return err
				}
			}

			maxVersion, err := cmd.Flags().GetInt64(flagMaximumVersion)
			if err != nil {
				return err
			}
			if maxVersion > 0 {
				// update latest version
				store := tsrocksdb.NewStoreWithDB(db, cfHandle)
				latestVersion, err := store.GetLatestVersion()
				if err != nil {
					return err
				}
				if maxVersion > latestVersion {
					fmt.Println("update latest version to", maxVersion)
					if err := store.SetLatestVersion(maxVersion); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool(flagMoveFiles, false, "move sst files instead of copy them")
	cmd.Flags().Int64(flagMaximumVersion, 0, "Specify the maximum version covered by the ingested files, if it's bigger than existing recorded latest version, will update it.")
	return cmd
}
