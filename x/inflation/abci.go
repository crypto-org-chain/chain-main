package inflation

import (
	"context"
	"fmt"

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
		balance, err := k.GetAddressBalance(ctx, ba, denom)
		if err != nil {
			panic(fmt.Sprintf("could not get %s balance: %s", ba, err.Error()))
		}
		burned = burned.Add(balance)
	}
	totalsupply = totalsupply.Sub(burned)

	maxsupply := params.MaxSupply
	if maxsupply.IsPositive() && totalsupply.GT(maxsupply) {
		panic(fmt.Sprintf("the total supply has exceeded the maximum supply: %s > %s", totalsupply, maxsupply))
	}

	return nil
}
