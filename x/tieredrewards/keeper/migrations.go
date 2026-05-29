package keeper

import (
	v2 "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/migrations/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Migrator struct {
	keeper Keeper
}

func NewMigrator(k Keeper) Migrator {
	return Migrator{keeper: k}
}

func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	return v2.Migrate(ctx, m.keeper.Positions, m.keeper.accountKeeper, m.keeper)
}
