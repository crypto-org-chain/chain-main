package v2

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func LegacyDelegatorAddress(id uint64) string {
	return authtypes.NewModuleAddress(fmt.Sprintf("tieredrewards/position/%d", id)).String()
}

func Migrate(ctx context.Context, positions collections.Map[uint64, types.Position]) error {
	iter, err := positions.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	kvs, err := iter.KeyValues()
	if err != nil {
		return err
	}

	for _, kv := range kvs {
		pos := kv.Value
		pos.DelegatorAddress = LegacyDelegatorAddress(pos.Id)
		if err := positions.Set(ctx, pos.Id, pos); err != nil {
			return err
		}
	}
	return nil
}
