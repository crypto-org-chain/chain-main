package keeper

import (
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/crypto-org-chain/chain-main/v4/x/mintsupply/types"
)

// BeginBlocker will check the total supply does not exceed the maximum supply and returns an error if it does.
func (k *Keeper) BeginBlocker(ctx context.Context) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, telemetry.Now(), telemetry.MetricKeyBeginBlocker)

	if k.GetSupply(ctx).GT(k.GetMaxSupply(ctx)) {
		return errors.New("the total supply has exceeded the maximum supply")
	}

	return nil
}
