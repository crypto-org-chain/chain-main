package keeper

import (
	"context"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var _ types.MsgServer = msgServer{}

// msgServer is a wrapper of Keeper.
type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the tieredrewards MsgServer interface.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// authorizePositionOwner fetches the position and verifies msg.Owner matches.
func (ms msgServer) authorizePositionOwner(ctx context.Context, positionId uint64, owner string) (types.TierPosition, error) {
	position, err := ms.GetPosition(ctx, positionId)
	if err != nil {
		return types.TierPosition{}, errors.Wrapf(sdkerrors.ErrNotFound, "position %d: %v", positionId, err)
	}
	if position.Owner != owner {
		return types.TierPosition{}, errors.Wrapf(govtypes.ErrInvalidSigner, "not position owner; expected %s, got %s", position.Owner, owner)
	}
	return position, nil
}

// validateBondDenom validates that the coin has the correct bond denomination.
func (ms msgServer) validateBondDenom(ctx context.Context, coin sdk.Coin) (string, error) {
	bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return "", err
	}
	if coin.Denom != bondDenom {
		return "", errors.Wrapf(sdkerrors.ErrInvalidRequest, "expected denom %s, got %s", bondDenom, coin.Denom)
	}
	return bondDenom, nil
}

// delegateToValidator resolves the validator and delegates from the tier module account.
func (ms msgServer) delegateToValidator(ctx context.Context, validatorAddr string, amount math.Int) (math.LegacyDec, error) {
	valAddr, err := sdk.ValAddressFromBech32(validatorAddr)
	if err != nil {
		return math.LegacyDec{}, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}
	validator, err := ms.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return math.LegacyDec{}, errors.Wrapf(sdkerrors.ErrNotFound, "validator %s: %v", validatorAddr, err)
	}

	tierModuleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)
	newShares, err := ms.stakingKeeper.Delegate(ctx, tierModuleAddr, amount, stakingtypes.Unbonded, validator, true)
	if err != nil {
		return math.LegacyDec{}, errors.Wrap(err, "staking delegate failed")
	}
	return newShares, nil
}

// ---------------------------------------------------------------------------
// 1. UpdateParams
// ---------------------------------------------------------------------------

// UpdateParams updates the module parameters.
func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.authority != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, msg.Authority)
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	if err := ms.Params.Set(ctx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

// ---------------------------------------------------------------------------
// 2. LockTier (ADR-006 SS5.1)
// ---------------------------------------------------------------------------

func (ms msgServer) LockTier(ctx context.Context, msg *types.MsgLockTier) (*types.MsgLockTierResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}

	// Load params and validate tier.
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	tierDef, err := params.GetTierDefinition(msg.TierId)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "%s", err)
	}

	// Validate bond denom.
	if _, err := ms.validateBondDenom(ctx, msg.Amount); err != nil {
		return nil, err
	}

	// HIGH-2: Validate amount is positive.
	if !msg.Amount.Amount.IsPositive() {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	// Validate minimum lock amount.
	if msg.Amount.Amount.LT(tierDef.MinLockAmount) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "amount %s is below tier minimum %s", msg.Amount.Amount, tierDef.MinLockAmount)
	}

	// Transfer tokens from user to the tier module account.
	coins := sdk.NewCoins(msg.Amount)
	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, types.ModuleName, coins); err != nil {
		return nil, err
	}

	// Create position in the store (returns TierPosition directly - 6f).
	position, err := ms.CreatePosition(ctx, msg.Owner, msg.TierId, msg.Amount.Amount)
	if err != nil {
		return nil, err
	}

	// Optional: delegate immediately.
	if msg.Validator != "" {
		newShares, err := ms.delegateToValidator(ctx, msg.Validator, msg.Amount.Amount)
		if err != nil {
			return nil, err
		}

		position.Validator = msg.Validator
		position.DelegatedShares = newShares
		position.DelegatedAtTime = blockTime
		position.LastBonusAccrual = blockTime

		if err := ms.AddTierShares(ctx, msg.Validator, newShares); err != nil {
			return nil, err
		}
	}

	// Optional: trigger exit immediately.
	if msg.TriggerExitImmediately {
		position.ExitTriggeredAt = blockTime
		position.ExitUnlockTime = blockTime.Add(tierDef.ExitCommitmentDuration)
	}

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventTierLock{ //nolint:errcheck
		PositionId: position.PositionId,
		Owner:      msg.Owner,
		TierId:     msg.TierId,
		Amount:     msg.Amount.String(),
		Validator:  msg.Validator,
		Exiting:    msg.TriggerExitImmediately,
	})

	return &types.MsgLockTierResponse{PositionId: position.PositionId}, nil
}

// ---------------------------------------------------------------------------
// 3. CommitDelegationToTier (ADR-006 SS5.2)
// ---------------------------------------------------------------------------

func (ms msgServer) CommitDelegationToTier(ctx context.Context, msg *types.MsgCommitDelegationToTier) (*types.MsgCommitDelegationToTierResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}
	valAddr, err := sdk.ValAddressFromBech32(msg.Validator)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	// Load params and validate tier.
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	tierDef, err := params.GetTierDefinition(msg.TierId)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "%s", err)
	}

	// Validate bond denom.
	if _, err := ms.validateBondDenom(ctx, msg.Amount); err != nil {
		return nil, err
	}

	// Validate minimum lock amount.
	if msg.Amount.Amount.LT(tierDef.MinLockAmount) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "amount %s is below tier minimum %s", msg.Amount.Amount, tierDef.MinLockAmount)
	}
	if !msg.Amount.Amount.IsPositive() {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	// Get the user's existing delegation to the validator.
	delegation, err := ms.stakingKeeper.GetDelegation(ctx, ownerAddr, valAddr)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrNotFound, "no delegation from %s to %s: %v", msg.Owner, msg.Validator, err)
	}

	// Look up validator to convert amount to shares.
	validator, err := ms.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrNotFound, "validator %s: %v", msg.Validator, err)
	}

	// Convert the requested token amount to delegation shares.
	sharesToTransfer, err := validator.SharesFromTokens(msg.Amount.Amount)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert tokens to shares")
	}

	// Ensure the user has enough shares.
	if sharesToTransfer.GT(delegation.Shares) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest,
			"insufficient delegation shares: need %s, have %s", sharesToTransfer, delegation.Shares)
	}

	// Transfer the delegation from user to tier module.
	tierModuleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)
	if err := ms.TransferDelegation(ctx, ownerAddr, tierModuleAddr, valAddr, sharesToTransfer); err != nil {
		return nil, errors.Wrap(err, "transfer delegation failed")
	}

	// Create position (returns TierPosition directly - 6f).
	position, err := ms.CreatePosition(ctx, msg.Owner, msg.TierId, msg.Amount.Amount)
	if err != nil {
		return nil, err
	}

	position.Validator = msg.Validator
	position.DelegatedShares = sharesToTransfer
	position.DelegatedAtTime = blockTime
	position.LastBonusAccrual = blockTime

	if err := ms.AddTierShares(ctx, msg.Validator, sharesToTransfer); err != nil {
		return nil, err
	}

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventCommitDelegationToTier{ //nolint:errcheck
		PositionId:   position.PositionId,
		Owner:        msg.Owner,
		TierId:       msg.TierId,
		Validator:    msg.Validator,
		AmountLocked: msg.Amount.Amount.String(),
	})

	return &types.MsgCommitDelegationToTierResponse{PositionId: position.PositionId}, nil
}

// ---------------------------------------------------------------------------
// 4. AddToTierPosition (ADR-006 SS5.3)
// ---------------------------------------------------------------------------

func (ms msgServer) AddToTierPosition(ctx context.Context, msg *types.MsgAddToTierPosition) (*types.MsgAddToTierPositionResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}

	// Auth check + fetch position.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Reject if unbonding (check before exiting since unbonding implies exiting).
	if position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is unbonding; cannot add tokens", msg.PositionId)
	}

	// Reject if exiting.
	if IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is exiting; cannot add tokens", msg.PositionId)
	}

	// Validate bond denom.
	if _, err := ms.validateBondDenom(ctx, msg.Amount); err != nil {
		return nil, err
	}
	if !msg.Amount.Amount.IsPositive() {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}

	// Transfer tokens from user to module.
	coins := sdk.NewCoins(msg.Amount)
	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, types.ModuleName, coins); err != nil {
		return nil, err
	}

	// If position is actively delegated, settle bonus (Option B: fair) then delegate the new tokens.
	if position.Validator != "" {
		// Compute and pay accrued bonus on the CURRENT AmountLocked before adding.
		params, err := ms.Params.Get(ctx)
		if err != nil {
			return nil, err
		}
		tierDef, err := params.GetTierDefinition(position.TierId)
		if err != nil {
			return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "%s", err)
		}
		bonus, err := ms.CalculateBonus(ctx, position, tierDef)
		if err != nil {
			return nil, errors.Wrap(err, "failed to calculate bonus on add")
		}
		if bonus.IsPositive() {
			bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
			if err != nil {
				return nil, err
			}
			// Cap to pool balance.
			poolAddr := ms.accountKeeper.GetModuleAddress(types.TierPoolName)
			poolBalance := ms.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)
			payoutAmt := bonus
			if poolBalance.Amount.LT(bonus) {
				payoutAmt = poolBalance.Amount
			}
			if payoutAmt.IsPositive() {
				bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, payoutAmt))
				if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.TierPoolName, ownerAddr, bonusCoins); err != nil {
					return nil, errors.Wrap(err, "failed to settle bonus on add")
				}
			}
		}
		// Always advance LastBonusAccrual when adding tokens, even if the bonus
		// was only partially paid. Unlike WithdrawTierRewards (which preserves
		// LastBonusAccrual on partial payment), AddToTierPosition increases
		// AmountLocked — if we preserved LastBonusAccrual, the next bonus
		// calculation would retroactively apply the higher amount to the unpaid
		// period, allowing users to inflate their bonus by adding tokens after
		// a partial payment.
		position.LastBonusAccrual = blockTime

		newShares, err := ms.delegateToValidator(ctx, position.Validator, msg.Amount.Amount)
		if err != nil {
			return nil, err
		}
		position.DelegatedShares = position.DelegatedShares.Add(newShares)

		if err := ms.AddTierShares(ctx, position.Validator, newShares); err != nil {
			return nil, err
		}
	}

	// Increase locked amount.
	position.AmountLocked = position.AmountLocked.Add(msg.Amount.Amount)

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventAddToTierPosition{ //nolint:errcheck
		PositionId:  msg.PositionId,
		Owner:       msg.Owner,
		AmountAdded: msg.Amount.Amount.String(),
		NewTotal:    position.AmountLocked.String(),
	})

	return &types.MsgAddToTierPositionResponse{}, nil
}

// ---------------------------------------------------------------------------
// 5. TierDelegate (ADR-006 SS5.4)
// ---------------------------------------------------------------------------

func (ms msgServer) TierDelegate(ctx context.Context, msg *types.MsgTierDelegate) (*types.MsgTierDelegateResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Reject unbonding or exiting positions first (more specific errors).
	// Note: unbonding positions always have Validator set (cleared by EndBlocker),
	// so check these before the "already delegated" guard.
	if position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is unbonding; cannot delegate", msg.PositionId)
	}
	if IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is exiting; cannot delegate", msg.PositionId)
	}

	// Require position NOT already delegated.
	if position.Validator != "" {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is already delegated to %s", msg.PositionId, position.Validator)
	}

	newShares, err := ms.delegateToValidator(ctx, msg.Validator, position.AmountLocked)
	if err != nil {
		return nil, err
	}

	position.Validator = msg.Validator
	position.DelegatedShares = newShares
	position.DelegatedAtTime = blockTime
	position.LastBonusAccrual = blockTime

	if err := ms.AddTierShares(ctx, msg.Validator, newShares); err != nil {
		return nil, err
	}

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventTierDelegate{ //nolint:errcheck
		PositionId: msg.PositionId,
		Owner:      msg.Owner,
		Validator:  msg.Validator,
	})

	return &types.MsgTierDelegateResponse{}, nil
}

// ---------------------------------------------------------------------------
// 6. TierUndelegate (ADR-006 SS5.4)
// ---------------------------------------------------------------------------

func (ms msgServer) TierUndelegate(ctx context.Context, msg *types.MsgTierUndelegate) (*types.MsgTierUndelegateResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Require delegated.
	if position.Validator == "" {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is not delegated", msg.PositionId)
	}

	// Require exit triggered.
	if !IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d has not triggered exit; must trigger exit before undelegating", msg.PositionId)
	}

	// Reject if already unbonding (double-call protection).
	if position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is already unbonding", msg.PositionId)
	}

	valAddr, err := sdk.ValAddressFromBech32(position.Validator)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	tierModuleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)
	// CRIT-2: Capture completionTime from Undelegate return value.
	completionTime, _, err := ms.stakingKeeper.Undelegate(ctx, tierModuleAddr, valAddr, position.DelegatedShares)
	if err != nil {
		return nil, errors.Wrap(err, "staking undelegate failed")
	}

	if err := ms.SubTierShares(ctx, position.Validator, position.DelegatedShares); err != nil {
		return nil, err
	}

	position.IsUnbonding = true
	position.UnbondingCompletionTime = completionTime
	position.DelegatedShares = math.LegacyZeroDec()

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Track this position in the unbonding index for efficient EndBlocker lookup.
	if err := ms.UnbondingPositions.Set(ctx, position.PositionId, completionTime.Unix()); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventTierUndelegate{ //nolint:errcheck
		PositionId: msg.PositionId,
		Owner:      msg.Owner,
	})

	return &types.MsgTierUndelegateResponse{}, nil
}

// ---------------------------------------------------------------------------
// 7. TierRedelegate (ADR-006 SS5.4)
// ---------------------------------------------------------------------------

func (ms msgServer) TierRedelegate(ctx context.Context, msg *types.MsgTierRedelegate) (*types.MsgTierRedelegateResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Require delegated.
	if position.Validator == "" {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is not delegated", msg.PositionId)
	}

	// Block redelegation when position is unbonding (check before exiting since unbonding implies exiting).
	if position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is unbonding; cannot redelegate", msg.PositionId)
	}

	// HIGH-3: Block redelegation when position is exiting.
	if IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is exiting; cannot redelegate", msg.PositionId)
	}

	// Block self-redelegation (same source and destination validator).
	if position.Validator == msg.DstValidator {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is already delegated to %s; cannot redelegate to same validator", msg.PositionId, msg.DstValidator)
	}

	srcValAddr, err := sdk.ValAddressFromBech32(position.Validator)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid source validator address: %s", err)
	}
	dstValAddr, err := sdk.ValAddressFromBech32(msg.DstValidator)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid destination validator address: %s", err)
	}

	tierModuleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)

	// Record previous destination shares to compute new shares after redelegation.
	var prevDstShares math.LegacyDec
	if dstDel, err := ms.stakingKeeper.GetDelegation(ctx, tierModuleAddr, dstValAddr); err == nil {
		prevDstShares = dstDel.GetShares()
	} else {
		prevDstShares = math.LegacyZeroDec()
	}

	oldSrcShares := position.DelegatedShares

	_, err = ms.stakingKeeper.BeginRedelegation(ctx, tierModuleAddr, srcValAddr, dstValAddr, position.DelegatedShares)
	if err != nil {
		return nil, errors.Wrap(err, "staking redelegate failed")
	}

	// Compute the new shares on the destination validator.
	dstDel, err := ms.stakingKeeper.GetDelegation(ctx, tierModuleAddr, dstValAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query destination delegation after redelegation")
	}
	newShares := dstDel.GetShares().Sub(prevDstShares)

	// Update TotalTierShares: subtract from source, add to destination.
	if err := ms.SubTierShares(ctx, position.Validator, oldSrcShares); err != nil {
		return nil, err
	}
	if err := ms.AddTierShares(ctx, msg.DstValidator, newShares); err != nil {
		return nil, err
	}

	position.Validator = msg.DstValidator
	position.DelegatedShares = newShares

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventTierRedelegate{ //nolint:errcheck
		PositionId:   msg.PositionId,
		Owner:        msg.Owner,
		DstValidator: msg.DstValidator,
	})

	return &types.MsgTierRedelegateResponse{}, nil
}

// ---------------------------------------------------------------------------
// 8. TriggerExitFromTier (ADR-006 SS5.5)
// ---------------------------------------------------------------------------

func (ms msgServer) TriggerExitFromTier(ctx context.Context, msg *types.MsgTriggerExitFromTier) (*types.MsgTriggerExitFromTierResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Reject if already exiting.
	if IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d has already triggered exit", msg.PositionId)
	}

	// Get tier definition for exit commitment duration.
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	tierDef, err := params.GetTierDefinition(position.TierId)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "%s", err)
	}

	position.ExitTriggeredAt = blockTime
	position.ExitUnlockTime = blockTime.Add(tierDef.ExitCommitmentDuration)

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventTriggerExit{ //nolint:errcheck
		PositionId:     msg.PositionId,
		Owner:          msg.Owner,
		ExitUnlockTime: position.ExitUnlockTime.UTC().Format(time.RFC3339),
	})

	return &types.MsgTriggerExitFromTierResponse{}, nil
}

// ---------------------------------------------------------------------------
// 9. WithdrawFromTier (ADR-006 SS5.6)
// ---------------------------------------------------------------------------

func (ms msgServer) WithdrawFromTier(ctx context.Context, msg *types.MsgWithdrawFromTier) (*types.MsgWithdrawFromTierResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Require exit triggered.
	if !IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d has not triggered exit", msg.PositionId)
	}

	// Require exit commitment elapsed.
	if blockTime.Before(position.ExitUnlockTime) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest,
			"position %d exit not yet unlocked; unlocks at %s, current time %s",
			msg.PositionId, position.ExitUnlockTime.UTC().Format(time.RFC3339), blockTime.UTC().Format(time.RFC3339))
	}

	// If was delegated, must have already undelegated and unbonding must be complete.
	if position.Validator != "" {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest,
			"position %d is still delegated to %s; must undelegate first", msg.PositionId, position.Validator)
	}
	if position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest,
			"position %d is still unbonding; wait for unbonding to complete", msg.PositionId)
	}

	// MED-4: Check module balance before sending.
	bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}

	// Build the full coins to return: locked amount + any pending base rewards.
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, position.AmountLocked))
	if position.PendingBaseRewards.IsAllPositive() {
		coins = coins.Add(position.PendingBaseRewards...)
	}

	moduleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)
	for _, coin := range coins {
		bal := ms.bankKeeper.GetBalance(ctx, moduleAddr, coin.Denom)
		if bal.Amount.LT(coin.Amount) {
			return nil, errors.Wrapf(sdkerrors.ErrInsufficientFunds,
				"module account has %s %s, need %s to return tokens", bal.Amount, coin.Denom, coin.Amount)
		}
	}

	// Send tokens from module back to owner.
	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, coins); err != nil {
		return nil, err
	}

	// Delete position.
	if err := ms.DeletePosition(ctx, msg.PositionId); err != nil {
		return nil, err
	}
	// Defensive: remove any stale UnbondingPositions entry (should already be
	// cleaned up by EndBlocker, but ensures no orphaned map entries).
	_ = ms.UnbondingPositions.Remove(ctx, msg.PositionId)

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventWithdrawFromTier{ //nolint:errcheck
		PositionId: msg.PositionId,
		Owner:      msg.Owner,
	})

	return &types.MsgWithdrawFromTierResponse{}, nil
}

// ---------------------------------------------------------------------------
// 10. WithdrawTierRewards (ADR-006 SS5.7) — CRIT-1: Fair reward attribution
// ---------------------------------------------------------------------------

func (ms msgServer) WithdrawTierRewards(ctx context.Context, msg *types.MsgWithdrawTierRewards) (*types.MsgWithdrawTierRewardsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()

	ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// Require actively delegated (not unbonding).
	if position.Validator == "" || position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is not actively delegated; no rewards to withdraw", msg.PositionId)
	}

	valAddr, err := sdk.ValAddressFromBech32(position.Validator)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	tierModuleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)

	// Phase 1: Withdraw the full delegation rewards from distribution to the tier module.
	fullBaseRewards, err := ms.distributionKeeper.WithdrawDelegationRewards(ctx, tierModuleAddr, valAddr)
	if err != nil {
		return nil, errors.Wrap(err, "withdraw delegation rewards failed")
	}

	// Phase 2: CRIT-1 — Fair attribution across ALL positions on this validator.
	// Use the stored TotalTierShares instead of iterating all positions.
	totalTierShares, err := ms.GetTotalTierShares(ctx, position.Validator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get total tier shares")
	}

	// Collect sibling positions using the validator index instead of full table scan.
	var siblingPositions []types.TierPosition
	valIter, err := ms.Positions.Indexes.Validator.MatchExact(ctx, position.Validator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to iterate positions by validator")
	}
	defer valIter.Close()

	for ; valIter.Valid(); valIter.Next() {
		pk, err := valIter.PrimaryKey()
		if err != nil {
			return nil, err
		}
		pos, err := ms.Positions.Get(ctx, pk)
		if err != nil {
			return nil, err
		}
		if !pos.DelegatedShares.IsNil() && pos.DelegatedShares.IsPositive() {
			siblingPositions = append(siblingPositions, pos)
		}
	}

	var positionBaseRewards sdk.Coins
	if totalTierShares.IsPositive() && !position.DelegatedShares.IsNil() && position.DelegatedShares.IsPositive() {
		// Attribute rewards to ALL sibling positions.
		for i, sibling := range siblingPositions {
			fraction := sibling.DelegatedShares.Quo(totalTierShares)
			var siblingRewards sdk.Coins
			for _, coin := range fullBaseRewards {
				attributed := fraction.MulInt(coin.Amount).TruncateInt()
				if attributed.IsPositive() {
					siblingRewards = siblingRewards.Add(sdk.NewCoin(coin.Denom, attributed))
				}
			}

			if sibling.PositionId == position.PositionId {
				// This is the calling position — collect its share plus any pending.
				positionBaseRewards = siblingRewards
			} else if siblingRewards.IsAllPositive() {
				// Other position — accumulate as pending base rewards.
				sibling.PendingBaseRewards = sibling.PendingBaseRewards.Add(siblingRewards...)
				siblingPositions[i] = sibling
				if err := ms.SetPosition(ctx, sibling); err != nil {
					return nil, errors.Wrapf(err, "failed to set pending rewards for position %d", sibling.PositionId)
				}
			}
		}
	}

	// Add any previously accumulated pending base rewards for the caller.
	if position.PendingBaseRewards.IsAllPositive() {
		positionBaseRewards = positionBaseRewards.Add(position.PendingBaseRewards...)
		position.PendingBaseRewards = nil // Clear after collecting.
	}

	// Send the position's share of base rewards from the module to the owner.
	if positionBaseRewards.IsAllPositive() {
		if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, positionBaseRewards); err != nil {
			return nil, errors.Wrap(err, "failed to send base rewards to owner")
		}
	}

	// Phase 3: Calculate and pay tier bonus.
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	tierDef, err := params.GetTierDefinition(position.TierId)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "%s", err)
	}

	bonus, err := ms.CalculateBonus(ctx, position, tierDef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate bonus")
	}

	var bonusRewards sdk.Coins
	bonusFullyPaid := true
	if bonus.IsPositive() {
		bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
		if err != nil {
			return nil, err
		}
		bonusCoin := sdk.NewCoin(bondDenom, bonus)
		bonusRewards = sdk.NewCoins(bonusCoin)

		// Cap bonus to available tier pool balance (ADR §10).
		poolAddr := ms.accountKeeper.GetModuleAddress(types.TierPoolName)
		poolBalance := ms.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)
		if poolBalance.Amount.IsZero() {
			bonusRewards = nil
			bonusFullyPaid = false
		} else if poolBalance.Amount.LT(bonus) {
			bonusCoin = sdk.NewCoin(bondDenom, poolBalance.Amount)
			bonusRewards = sdk.NewCoins(bonusCoin)
			bonusFullyPaid = false
		}

		// Send bonus from tier pool to the owner (skip if pool was empty).
		if bonusRewards.IsAllPositive() {
			if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.TierPoolName, ownerAddr, bonusRewards); err != nil {
				return nil, errors.Wrap(err, "failed to send bonus rewards to owner")
			}
		}
	}

	// Only advance LastBonusAccrual if bonus was fully paid. When the pool is
	// insufficient, keep LastBonusAccrual unchanged so the unpaid portion can
	// be claimed later when the pool is funded again.
	if bonusFullyPaid {
		position.LastBonusAccrual = blockTime
	}
	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventWithdrawTierRewards{ //nolint:errcheck
		PositionId:  msg.PositionId,
		Owner:       msg.Owner,
		BaseAmount:  positionBaseRewards.String(),
		BonusAmount: bonusRewards.String(),
	})

	return &types.MsgWithdrawTierRewardsResponse{
		BaseRewards:  positionBaseRewards,
		BonusRewards: bonusRewards,
	}, nil
}

// ---------------------------------------------------------------------------
// 11. FundTierPool (ADR-006 SS5.8)
// ---------------------------------------------------------------------------

func (ms msgServer) FundTierPool(ctx context.Context, msg *types.MsgFundTierPool) (*types.MsgFundTierPoolResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address: %s", err)
	}

	if !msg.Amount.IsValid() || msg.Amount.IsZero() {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid fund amount: %s", msg.Amount)
	}

	// 6i: Restrict to bond denom only.
	bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	for _, coin := range msg.Amount {
		if coin.Denom != bondDenom {
			return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest,
				"only bond denom %s is accepted; got %s", bondDenom, coin.Denom)
		}
	}

	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, senderAddr, types.TierPoolName, msg.Amount); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventFundTierPool{ //nolint:errcheck
		Sender: msg.Sender,
		Amount: msg.Amount.String(),
	})

	return &types.MsgFundTierPoolResponse{}, nil
}

// ---------------------------------------------------------------------------
// 12. TransferTierPosition (ADR-006 SS12, optional)
// ---------------------------------------------------------------------------

func (ms msgServer) TransferTierPosition(ctx context.Context, msg *types.MsgTransferTierPosition) (*types.MsgTransferTierPositionResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate new owner address.
	_, err := sdk.AccAddressFromBech32(msg.NewOwner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid new_owner address: %s", err)
	}

	// Auth check.
	position, err := ms.authorizePositionOwner(ctx, msg.PositionId, msg.Owner)
	if err != nil {
		return nil, err
	}

	// MED-2: Reject self-transfer.
	if msg.Owner == msg.NewOwner {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "cannot transfer position to self")
	}

	// MED-2: Reject transfer when exiting.
	if IsPositionExiting(position) {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is exiting; cannot transfer", msg.PositionId)
	}

	// MED-2: Reject transfer when unbonding.
	if position.IsUnbonding {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "position %d is unbonding; cannot transfer", msg.PositionId)
	}

	oldOwner := position.Owner
	oldOwnerAddr, err := sdk.AccAddressFromBech32(oldOwner)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}

	blockTime := sdkCtx.BlockTime()

	// Settle accrued bonus to old owner before transfer.
	if IsPositionDelegated(position) {
		params, err := ms.Params.Get(ctx)
		if err != nil {
			return nil, err
		}
		tierDef, err := params.GetTierDefinition(position.TierId)
		if err != nil {
			return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "%s", err)
		}
		bonus, err := ms.CalculateBonus(ctx, position, tierDef)
		if err != nil {
			return nil, errors.Wrap(err, "failed to calculate bonus on transfer")
		}
		if bonus.IsPositive() {
			bondDenom, err := ms.stakingKeeper.BondDenom(ctx)
			if err != nil {
				return nil, err
			}
			poolAddr := ms.accountKeeper.GetModuleAddress(types.TierPoolName)
			poolBalance := ms.bankKeeper.GetBalance(ctx, poolAddr, bondDenom)
			payoutAmt := bonus
			if poolBalance.Amount.LT(bonus) {
				payoutAmt = poolBalance.Amount
			}
			if payoutAmt.IsPositive() {
				bonusCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, payoutAmt))
				if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.TierPoolName, oldOwnerAddr, bonusCoins); err != nil {
					return nil, errors.Wrap(err, "failed to settle bonus on transfer")
				}
			}
		}
		// Always advance LastBonusAccrual on transfer, even if bonus was only
		// partially paid. Unlike WithdrawTierRewards (which preserves accrual
		// time for retry), transfer is a one-way ownership change — carrying
		// forward unpaid bonus to a new owner would create accounting complexity.
		position.LastBonusAccrual = blockTime

		// Settle base staking rewards from the distribution module to prevent
		// the new owner from receiving rewards earned by the old owner.
		valAddr, err := sdk.ValAddressFromBech32(position.Validator)
		if err != nil {
			return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
		}
		tierModuleAddr := ms.accountKeeper.GetModuleAddress(types.ModuleName)
		fullBaseRewards, err := ms.distributionKeeper.WithdrawDelegationRewards(ctx, tierModuleAddr, valAddr)
		if err != nil {
			// If no rewards accumulated yet, this may error; not fatal.
			ms.Logger(ctx).Debug("withdraw delegation rewards on transfer", "err", err)
			fullBaseRewards = nil
		}

		if fullBaseRewards.IsAllPositive() {
			totalTierShares, err := ms.GetTotalTierShares(ctx, position.Validator)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get total tier shares")
			}
			if totalTierShares.IsPositive() {
				// Attribute rewards across all sibling positions on this validator.
				valIter, err := ms.Positions.Indexes.Validator.MatchExact(ctx, position.Validator)
				if err != nil {
					return nil, errors.Wrap(err, "failed to iterate positions by validator")
				}
				defer valIter.Close()

				for ; valIter.Valid(); valIter.Next() {
					pk, err := valIter.PrimaryKey()
					if err != nil {
						return nil, err
					}
					sib, err := ms.Positions.Get(ctx, pk)
					if err != nil {
						return nil, err
					}
					if sib.DelegatedShares.IsNil() || !sib.DelegatedShares.IsPositive() {
						continue
					}
					fraction := sib.DelegatedShares.Quo(totalTierShares)
					var sibRewards sdk.Coins
					for _, coin := range fullBaseRewards {
						attributed := fraction.MulInt(coin.Amount).TruncateInt()
						if attributed.IsPositive() {
							sibRewards = sibRewards.Add(sdk.NewCoin(coin.Denom, attributed))
						}
					}
					if sibRewards.IsAllPositive() {
						sib.PendingBaseRewards = sib.PendingBaseRewards.Add(sibRewards...)
						if sib.PositionId == position.PositionId {
							// Update our local copy so PendingBaseRewards is paid to old owner below.
							position.PendingBaseRewards = sib.PendingBaseRewards
						} else {
							if err := ms.SetPosition(ctx, sib); err != nil {
								return nil, errors.Wrapf(err, "failed to set pending rewards for position %d", sib.PositionId)
							}
						}
					}
				}
			}
		}
	}

	// Pay out accumulated PendingBaseRewards to old owner.
	if position.PendingBaseRewards.IsAllPositive() {
		if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, oldOwnerAddr, position.PendingBaseRewards); err != nil {
			return nil, errors.Wrap(err, "failed to settle pending base rewards on transfer")
		}
		position.PendingBaseRewards = nil
	}

	position.Owner = msg.NewOwner

	if err := ms.SetPosition(ctx, position); err != nil {
		return nil, err
	}

	// Emit event.
	sdkCtx.EventManager().EmitTypedEvent(&types.EventTransferTierPosition{ //nolint:errcheck
		PositionId: msg.PositionId,
		OldOwner:   oldOwner,
		NewOwner:   msg.NewOwner,
	})

	return &types.MsgTransferTierPositionResponse{}, nil
}
