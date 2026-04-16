package inflation_test

import (
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/inflation"
	"github.com/crypto-org-chain/chain-main/v8/x/inflation/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// TestBeginBlocker_UnlimitedSupply verifies that BeginBlocker does NOT panic
// when max_supply is 0 (unlimited supply mode, the default).
func TestBeginBlocker_UnlimitedSupply(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	// Default params have max_supply = 0 (unlimited)
	params, err := app.InflationKeeper.GetParams(ctx)
	require.NoError(t, err)
	require.True(t, params.MaxSupply.IsZero())

	require.NotPanics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}

// TestBeginBlocker_SupplyExactlyAtMax verifies that the chain does NOT halt
// when total supply equals max supply exactly (the check is strictly GT).
func TestBeginBlocker_SupplyExactlyAtMax(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, _, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	// Set max_supply = total_supply (exactly at the boundary)
	params := types.DefaultParams()
	params.MaxSupply = totalSupply
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	// GT, not GTE — should NOT panic
	require.NotPanics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}

// TestBeginBlocker_SupplyBelowMax verifies no panic when supply is safely
// below the cap.
func TestBeginBlocker_SupplyBelowMax(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, _, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	// max_supply well above total
	params := types.DefaultParams()
	params.MaxSupply = totalSupply.Add(math.NewInt(1_000_000_000))
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}

// TestBeginBlocker_SupplyExceedsMax verifies that the chain panics when the
// total supply exceeds the maximum supply.
func TestBeginBlocker_SupplyExceedsMax(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, _, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	// Set max_supply 1 below total supply
	params := types.DefaultParams()
	params.MaxSupply = totalSupply.Sub(math.NewInt(1))
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	require.PanicsWithValue(t,
		"the total supply has exceeded the maximum supply: "+totalSupply.String()+" > "+params.MaxSupply.String(),
		func() {
			err := inflation.BeginBlocker(ctx, app.InflationKeeper)
			require.NoError(t, err)
		},
	)
}

// TestBeginBlocker_BurnedAddressReducesEffectiveSupply verifies that burned
// address balances are subtracted from total supply before the max check.
// Raw total > max, but effective (total − burned) = max → no panic.
func TestBeginBlocker_BurnedAddressReducesEffectiveSupply(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, denom, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	// Fund a "burned" account — this increases the raw total supply
	burnedAddr := sdk.AccAddress([]byte("burned_addr_12345678"))
	burnedAmount := math.NewInt(1_000_000)
	err = banktestutil.FundAccount(ctx, app.BankKeeper, burnedAddr, sdk.NewCoins(sdk.NewCoin(denom, burnedAmount)))
	require.NoError(t, err)

	params := types.DefaultParams()
	params.MaxSupply = totalSupply
	params.BurnedAddresses = []string{burnedAddr.String()}
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	// current total supply is now totalSupply + burnedAmount
	// circulating supply is now totalSupply - burnedAmount which is equals to the total supply set in params
	// not GT so should not panic
	require.NotPanics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}

// TestBeginBlocker_MultipleBurnedAddresses verifies that multiple burned
// address balances are all subtracted before the max check.
func TestBeginBlocker_MultipleBurnedAddresses(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, denom, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	burnedAddr1 := sdk.AccAddress([]byte("burned_addr1_1234567"))
	burnedAddr2 := sdk.AccAddress([]byte("burned_addr2_1234567"))
	burnedAmount1 := math.NewInt(500_000)
	burnedAmount2 := math.NewInt(300_000)

	err = banktestutil.FundAccount(ctx, app.BankKeeper, burnedAddr1, sdk.NewCoins(sdk.NewCoin(denom, burnedAmount1)))
	require.NoError(t, err)
	err = banktestutil.FundAccount(ctx, app.BankKeeper, burnedAddr2, sdk.NewCoins(sdk.NewCoin(denom, burnedAmount2)))
	require.NoError(t, err)

	// Raw total = totalSupply + 800_000
	params := types.DefaultParams()
	params.MaxSupply = totalSupply
	params.BurnedAddresses = []string{burnedAddr1.String(), burnedAddr2.String()}
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}

// TestBeginBlocker_BurnedAddressNotEnough verifies that the chain panics
// when burned addresses don't bring effective supply below max.
func TestBeginBlocker_BurnedAddressNotEnough(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, denom, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	// Fund a burned account with a small amount
	burnedAddr := sdk.AccAddress([]byte("burned_addr_12345678"))
	burnedAmount := math.NewInt(100)
	err = banktestutil.FundAccount(ctx, app.BankKeeper, burnedAddr, sdk.NewCoins(sdk.NewCoin(denom, burnedAmount)))
	require.NoError(t, err)

	// Raw total = totalSupply + 100
	// Set max = totalSupply − 1 → effective = totalSupply + 100 − 100 = totalSupply > max → panic
	params := types.DefaultParams()
	params.MaxSupply = totalSupply.Sub(math.NewInt(1))
	params.BurnedAddresses = []string{burnedAddr.String()}
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	require.Panics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}

// TestBeginBlocker_NoBurnedAddresses verifies the check works correctly
// with a positive max supply but no burned addresses configured.
func TestBeginBlocker_NoBurnedAddresses(t *testing.T) {
	app := testutil.Setup(false, nil)
	ctx := app.BaseApp.NewContext(false).WithBlockHeader(tmproto.Header{ChainID: testutil.ChainID})

	totalSupply, _, err := app.InflationKeeper.GetSupplyAndDenom(ctx)
	require.NoError(t, err)

	// max above total, no burned addresses
	params := types.DefaultParams()
	params.MaxSupply = totalSupply.Add(math.NewInt(1))
	params.BurnedAddresses = []string{}
	err = app.InflationKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		err := inflation.BeginBlocker(ctx, app.InflationKeeper)
		require.NoError(t, err)
	})
}
