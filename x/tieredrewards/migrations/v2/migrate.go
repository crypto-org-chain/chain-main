package v2

import (
	"context"
	"fmt"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
)

type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

type PositionForceExiter interface {
	ForceFullExitWithDelegation(ctx context.Context, posID uint64) error
}

func LegacyDelegatorAddress(id uint64) string {
	return authtypes.NewModuleAddress(fmt.Sprintf("tieredrewards/position/%d", id)).String()
}

func Migrate(
	ctx context.Context,
	positions collections.Map[uint64, types.Position],
	ak AccountKeeper,
	pk PositionForceExiter,
) error {
	if err := backfillDelegatorAddress(ctx, positions); err != nil {
		return fmt.Errorf("backfill delegator address: %w", err)
	}
	if err := exitVestedAccountsPositions(ctx, positions, ak, pk); err != nil {
		return fmt.Errorf("exit vested accounts positions: %w", err)
	}
	return nil
}

func backfillDelegatorAddress(ctx context.Context, positions collections.Map[uint64, types.Position]) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("v8 migration: backfill delegator address")
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
		if _, err := sdk.AccAddressFromBech32(pos.DelegatorAddress); err != nil {
			return fmt.Errorf("backfill produced invalid delegator address for position %d: %w", pos.Id, err)
		}
		if err := positions.Set(ctx, pos.Id, pos); err != nil {
			return err
		}
	}
	sdkCtx.Logger().Info("v8 migration: delegator address backfilled", "count", len(kvs))
	return nil
}

func exitVestedAccountsPositions(
	ctx context.Context,
	positions collections.Map[uint64, types.Position],
	ak AccountKeeper,
	pk PositionForceExiter,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	var toExit []uint64
	if err := positions.Walk(ctx, nil, func(posID uint64, pos types.Position) (bool, error) {
		ownerAddr, err := sdk.AccAddressFromBech32(pos.Owner)
		if err != nil {
			return false, fmt.Errorf("parse owner address for position %d: %w", posID, err)
		}
		acc := ak.GetAccount(ctx, ownerAddr)
		if acc == nil {
			return false, fmt.Errorf("owner account not found for position %d: %s", posID, ownerAddr.String())
		}
		if _, ok := acc.(sdkvesting.VestingAccount); ok {
			toExit = append(toExit, posID)
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("walk positions: %w", err)
	}

	for _, posID := range toExit {
		sdkCtx.Logger().Info("v8 migration: force-exit vesting-owned position", "position_id", posID)
		if err := pk.ForceFullExitWithDelegation(ctx, posID); err != nil {
			return fmt.Errorf("force-exit position %d: %w", posID, err)
		}
	}

	sdkCtx.Logger().Info("v8 migration: vesting-owned positions exited", "count", len(toExit))
	return nil
}
