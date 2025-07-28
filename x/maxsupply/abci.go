package maxsupply

import (
	"context"
	"errors"

	"github.com/crypto-org-chain/chain-main/v4/x/maxsupply/keeper"
	"github.com/crypto-org-chain/chain-main/v4/x/maxsupply/types"

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

	totalBurnedDenom := math.NewInt(0)
	for _, ba := range params.BurnedAddresses {
		balance := k.GetAddressBalance(ctx, ba, denom)
		totalBurnedDenom.Add(balance)
	}
	totalsupply = totalsupply.Sub(totalBurnedDenom)

	maxsupply := params.MaxSupply
	if maxsupply.IsPositive() && totalsupply.GT(maxsupply) {
		return errors.New("the total supply has exceeded the maximum supply")
	}

	return nil
}
