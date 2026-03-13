package types

import (
	"context"
	"time"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type StakingKeeper interface {
	TotalBondedTokens(context.Context) (math.Int, error)
	BondDenom(ctx context.Context) (string, error)
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error)
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	GetDelegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.Delegation, error)
	Delegate(ctx context.Context, delAddr sdk.AccAddress, bondAmt math.Int, tokenSrc stakingtypes.BondStatus, validator stakingtypes.Validator, subtractAccount bool) (newShares math.LegacyDec, err error)
	Undelegate(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, sharesAmount math.LegacyDec) (time.Time, math.Int, error)
	BeginRedelegation(ctx context.Context, delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress, sharesAmount math.LegacyDec) (completionTime time.Time, err error)
	// SetDelegation and RemoveDelegation for TransferDelegation
	SetDelegation(ctx context.Context, delegation stakingtypes.Delegation) error
	RemoveDelegation(ctx context.Context, delegation stakingtypes.Delegation) error
}

type AccountKeeper interface {
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
	GetModuleAddress(moduleName string) sdk.AccAddress
}

type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

type MintKeeper interface {
	GetParams(ctx context.Context) (minttypes.Params, error)
}

type DistributionKeeper interface {
	GetCommunityTax(ctx context.Context) (math.LegacyDec, error)
	AllocateTokensToValidator(ctx context.Context, val stakingtypes.ValidatorI, tokens sdk.DecCoins) error
	WithdrawDelegationRewards(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (sdk.Coins, error)
}

// DistributionKeeperWithInit extends DistributionKeeper with InitializeDelegation.
// This is optional; the SDK fork must export InitializeDelegation for it to be available.
// TransferDelegation in keeper/position.go type-asserts to this interface at runtime
// so that reward tracking is correctly reinitialized after shares are moved between
// delegators. If the concrete keeper does not implement this, the transfer still
// succeeds but reward-per-share accounting may drift until the next full withdrawal.
type DistributionKeeperWithInit interface {
	DistributionKeeper
	InitializeDelegation(ctx context.Context, valAddr sdk.ValAddress, delAddr sdk.AccAddress) error
}
