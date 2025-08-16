package client

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/iavl"
	"github.com/spf13/cobra"
)

func PrintChangeSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print [plain-file]",
		Short: "Pretty-print the content of change set file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			noParseChangeset, err := cmd.Flags().GetBool(flagNoParseChangeset)
			if err != nil {
				return err
			}

			startVersion, err := cmd.Flags().GetInt64(flagStartVersion)
			if err != nil {
				return err
			}
			endVersion, err := cmd.Flags().GetInt64(flagEndVersion)
			if err != nil {
				return err
			}

			return withChangeSetFile(args[0], func(reader Reader) error {
				if noParseChangeset {
					// print the version numbers only
					_, err := IterateVersions(reader, func(version int64) (bool, error) {
						if version < startVersion {
							return true, nil
						}
						if endVersion > 0 && version >= endVersion {
							return false, nil
						}

						fmt.Printf("version: %d\n", version)
						return true, nil
					})
					return err
				}

				_, err := IterateChangeSets(reader, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
					if version < startVersion {
						return true, nil
					}
					if endVersion > 0 && version >= endVersion {
						return false, nil
					}
					fmt.Printf("version: %d\n", version)
					for _, pair := range changeSet.Pairs {
						js, err := json.Marshal(pair)
						if err != nil {
							return false, err
						}
						fmt.Println(string(js))
					}
					return true, nil
				})
				return err
			})
		},
	}
	cmd.Flags().Bool(flagNoParseChangeset, false, "if parse and output the change set content, otherwise only version numbers are outputted")
	cmd.Flags().Int64(flagStartVersion, 0, "Start of the version range to print")
	cmd.Flags().Int64(flagEndVersion, 0, "End(exclusive) of the version range to print, 0 means no end")
	return cmd
}
