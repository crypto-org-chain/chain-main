package client

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"

	"cosmossdk.io/errors"
	protoio "github.com/cosmos/gogoproto/io"
	"github.com/spf13/cobra"

	"cosmossdk.io/store/snapshots"
	"cosmossdk.io/store/snapshots/types"
	"github.com/cosmos/cosmos-sdk/server"

	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

// RestoreVersionDBCmd returns a command to restore a versiondb from local snapshot
func RestoreVersionDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore-versiondb <height> <format>",
		Short: "Restore initial versiondb from local snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)

			height, err := strconv.ParseUint(args[0], 10, 63)
			if err != nil {
				return err
			}
			format, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return err
			}

			store, err := server.GetSnapshotStore(ctx.Viper)
			if err != nil {
				return err
			}

			snapshot, chChunks, err := store.Load(height, uint32(format))
			if err != nil {
				return err
			}

			if snapshot == nil {
				return fmt.Errorf("snapshot doesn't exist, height: %d, format: %d", height, format)
			}

			streamReader, err := snapshots.NewStreamReader(chChunks)
			if err != nil {
				return err
			}
			defer streamReader.Close()

			home := ctx.Config.RootDir
			versionDB, err := tsrocksdb.NewStore(filepath.Join(home, "data", "versiondb"))
			if err != nil {
				return err
			}

			ch := make(chan versiondb.ImportEntry, 128)

			go func() {
				defer close(ch)

				if err := readSnapshotEntries(streamReader, ch); err != nil {
					ctx.Logger.Error("failed to read snapshot entries", "err", err)
				}
			}()

			return versionDB.Import(int64(height), ch)
		},
	}
	return cmd
}

// readSnapshotEntries reads key-value entries from protobuf reader and feed to the channel
func readSnapshotEntries(protoReader protoio.Reader, ch chan<- versiondb.ImportEntry) error {
	var (
		snapshotItem types.SnapshotItem
		storeKey     string
	)

loop:
	for {
		snapshotItem = types.SnapshotItem{}
		err := protoReader.ReadMsg(&snapshotItem)
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "invalid protobuf message")
		}

		switch item := snapshotItem.Item.(type) {
		case *types.SnapshotItem_Store:
			storeKey = item.Store.Name
		case *types.SnapshotItem_IAVL:
			if storeKey == "" {
				return errors.Wrap(err, "invalid protobuf message, store name is empty")
			}
			if item.IAVL.Height > math.MaxInt8 {
				return fmt.Errorf("node height %v cannot exceed %v",
					item.IAVL.Height, math.MaxInt8)
			}
			ch <- versiondb.ImportEntry{
				StoreKey: storeKey,
				Key:      item.IAVL.Key,
				Value:    item.IAVL.Value,
			}
		default:
			break loop
		}
	}

	return nil
}
