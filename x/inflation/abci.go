package inflation

import (
	"fmt"
	"context"

	"github.com/crypto-org-chain/chain-main/v8/x/inflation/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/telemetry"
)

// BeginBlocker will check the total supply does not exceed the maximum supply and returns an error if it does.
func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, telemetry.Now(), telemetry.MetricKeyBeginBlocker)
	params, err := k.GetParams(ctx)
	if err != nil {
		panic("could not get params: " + err.Error())
	}

	totalsupply, denom, err := k.GetSupplyAndDenom(ctx)
	if err != nil {
		panic("could not get supply and denom: " + err.Error())
	}

	burned := math.NewInt(0)
	for _, ba := range params.BurnedAddresses {
		balance := k.GetAddressBalance(ctx, ba, denom)
		burned = burned.Add(balance)
	}
	totalsupply = totalsupply.Sub(burned)

	maxsupply := params.MaxSupply
	if maxsupply.IsPositive() && totalsupply.GT(maxsupply) {
		return fmt.Errorf("the total supply has exceeded the maximum supply: %s > %s", totalsupply, maxsupply)
	}

	return nil
}
