package keeper

import (
	newsdkerrors "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-org-chain/chain-main/v4/config"
	"github.com/crypto-org-chain/chain-main/v4/x/supply/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestexported "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var (
	// ModuleAccounts defines the module accounts which will be queried to get liquid supply
	ModuleAccounts = []string{
		authtypes.FeeCollectorName,
		distrtypes.ModuleName,
		stakingtypes.BondedPoolName,
		stakingtypes.NotBondedPoolName,
		minttypes.ModuleName,
		govtypes.ModuleName,
	}
)

// Keeper for supply module
type Keeper struct {
	cdc           codec.BinaryCodec
	storeKey      storetypes.StoreKey
	bankKeeper    types.BankKeeper
	accountKeeper types.AccountKeeper
}

// NewKeeper returns a new keeper
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
	}
}

// FetchVestingAccounts iterates over all the accounts and returns addresses of all the vesting accounts
func (k Keeper) FetchVestingAccounts(ctx sdk.Context) types.VestingAccounts {
	var addresses []string

	k.accountKeeper.IterateAccounts(ctx, func(account authtypes.AccountI) bool {
		vacc, ok := account.(vestexported.VestingAccount)
		if ok {
			addresses = append(addresses, vacc.GetAddress().String())
		}
		return false
	})

	return types.VestingAccounts{
		Addresses: addresses,
	}
}

// SetVestingAccounts persists given vesting accounts
func (k Keeper) SetVestingAccounts(ctx sdk.Context, vestingAccounts types.VestingAccounts) {
	store := ctx.KVStore(k.storeKey)
	b := k.cdc.MustMarshal(&vestingAccounts)
	store.Set(types.VestingAccountsKey, b)
}

// GetVestingAccounts returns stored vesting accounts
func (k Keeper) GetVestingAccounts(ctx sdk.Context) types.VestingAccounts {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(types.VestingAccountsKey)

	if b == nil {
		return types.VestingAccounts{
			Addresses: []string{},
		}
	}

	var vestingAccounts types.VestingAccounts
	k.cdc.MustUnmarshal(b, &vestingAccounts)
	return vestingAccounts
}

// GetTotalSupply returns the current total supply in the system
func (k Keeper) GetTotalSupply(ctx sdk.Context) sdk.Coins {
	return sdk.NewCoins(k.bankKeeper.GetSupply(ctx, config.BaseCoinUnit))
}

// GetUnvestedSupply returns total unvested supply
func (k Keeper) GetUnvestedSupply(ctx sdk.Context) sdk.Coins {
	vestingAccounts := k.GetVestingAccounts(ctx)

	var lockedCoins sdk.Coins

	for _, vestingAccountAddress := range vestingAccounts.GetAddresses() {
		addr, err := sdk.AccAddressFromBech32(vestingAccountAddress)
		if err != nil {
			panic(err)
		}

		lockedCoins = lockedCoins.Add(k.bankKeeper.LockedCoins(ctx, addr)...)
	}

	return lockedCoins
}

// GetModuleAccountBalance returns the balance of a module account
func (k Keeper) GetModuleAccountBalance(ctx sdk.Context, moduleName string) sdk.Coins {
	addr := k.accountKeeper.GetModuleAddress(moduleName)

	if addr == nil {
		panic(newsdkerrors.Wrapf(sdkerrors.ErrUnknownAddress, "module account %s does not exist", moduleName))
	}

	return k.bankKeeper.GetAllBalances(ctx, addr)
}

// GetTotalModuleAccountBalance returns total balance of given module accounts
func (k Keeper) GetTotalModuleAccountBalance(ctx sdk.Context, moduleNames ...string) sdk.Coins {
	var balance sdk.Coins

	for _, moduleName := range moduleNames {
		balance = balance.Add(k.GetModuleAccountBalance(ctx, moduleName)...)
	}

	return balance
}

// GetLiquidSupply returns the total liquid supply in the system
func (k Keeper) GetLiquidSupply(ctx sdk.Context) sdk.Coins {
	totalSupply := k.GetTotalSupply(ctx)
	unvestedSupply := k.GetUnvestedSupply(ctx)
	moduleAccountBalance := k.GetTotalModuleAccountBalance(ctx, ModuleAccounts...)

	return totalSupply.Sub(unvestedSupply...).Sub(moduleAccountBalance...)
}
