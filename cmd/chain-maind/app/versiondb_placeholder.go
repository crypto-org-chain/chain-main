//go:build !rocksdb
// +build !rocksdb

package app

import (
	"github.com/spf13/cobra"
)

func ChangeSetCmd() *cobra.Command {
	return nil
}

func GetChangeSetCmd() *cobra.Command {
	return nil
}
