package client

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func ListDefaultStoresCmd(stores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default-stores",
		Short: "List the store names in current binary version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range stores {
				fmt.Println(name)
			}

			return nil
		},
	}
	return cmd
}

func GetStoresOrDefault(cmd *cobra.Command, defaultStores []string) ([]string, error) {
	stores, err := cmd.Flags().GetString(flagStores)
	if err != nil {
		return nil, err
	}
	if len(stores) == 0 {
		return defaultStores, nil
	}
	return strings.Split(stores, " "), nil
}
