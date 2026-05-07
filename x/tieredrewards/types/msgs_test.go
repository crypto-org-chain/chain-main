package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestMsgLockTier_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()
	validValidator := sdk.ValAddress([]byte("test_validator______")).String()

	tests := []struct {
		name        string
		msg         types.MsgLockTier
		wantErr     bool
		errContains string
	}{
		{
			name: "missing validator address",
			msg: types.MsgLockTier{
				Owner:  validOwner,
				Id:     1,
				Amount: sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid validator address",
		},
		{
			name: "valid with validator",
			msg: types.MsgLockTier{
				Owner:            validOwner,
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
				ValidatorAddress: validValidator,
			},
		},
		{
			name: "valid with trigger exit",
			msg: types.MsgLockTier{
				Owner:                  validOwner,
				Id:                     1,
				Amount:                 sdkmath.NewInt(1000),
				ValidatorAddress:       validValidator,
				TriggerExitImmediately: true,
			},
		},
		{
			name: "zero tier id",
			msg: types.MsgLockTier{
				Owner:            validOwner,
				Id:               0,
				Amount:           sdkmath.NewInt(1000),
				ValidatorAddress: validValidator,
			},
			wantErr:     true,
			errContains: "tier id must be non-zero",
		},
		{
			name: "invalid owner",
			msg: types.MsgLockTier{
				Owner:  "invalid",
				Id:     1,
				Amount: sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "empty owner",
			msg: types.MsgLockTier{
				Owner:  "",
				Id:     1,
				Amount: sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "zero amount",
			msg: types.MsgLockTier{
				Owner:  validOwner,
				Id:     1,
				Amount: sdkmath.ZeroInt(),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
		{
			name: "negative amount",
			msg: types.MsgLockTier{
				Owner:  validOwner,
				Id:     1,
				Amount: sdkmath.NewInt(-100),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
		{
			name: "invalid validator address",
			msg: types.MsgLockTier{
				Owner:            validOwner,
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
				ValidatorAddress: "invalid",
			},
			wantErr:     true,
			errContains: "invalid validator address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgCommitDelegationToTier_Validate(t *testing.T) {
	t.Parallel()

	validDelegator := sdk.AccAddress([]byte("test_delegator______")).String()
	validValidator := sdk.ValAddress([]byte("test_validator______")).String()

	tests := []struct {
		name        string
		msg         types.MsgCommitDelegationToTier
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: validDelegator,
				ValidatorAddress: validValidator,
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
			},
		},
		{
			name: "valid with trigger exit",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress:       validDelegator,
				ValidatorAddress:       validValidator,
				Id:                     1,
				Amount:                 sdkmath.NewInt(1000),
				TriggerExitImmediately: true,
			},
		},
		{
			name: "zero tier id",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: validDelegator,
				ValidatorAddress: validValidator,
				Id:               0,
				Amount:           sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "tier id must be non-zero",
		},
		{
			name: "invalid delegator",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: "invalid",
				ValidatorAddress: validValidator,
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid delegator address",
		},
		{
			name: "empty delegator",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: "",
				ValidatorAddress: validValidator,
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid delegator address",
		},
		{
			name: "empty validator",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: validDelegator,
				ValidatorAddress: "",
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid validator address",
		},
		{
			name: "invalid validator",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: validDelegator,
				ValidatorAddress: "invalid",
				Id:               1,
				Amount:           sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid validator address",
		},
		{
			name: "zero amount",
			msg: types.MsgCommitDelegationToTier{
				DelegatorAddress: validDelegator,
				ValidatorAddress: validValidator,
				Id:               1,
				Amount:           sdkmath.ZeroInt(),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgTierUndelegate_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgTierUndelegate
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgTierUndelegate{
				Owner:      validOwner,
				PositionId: 1,
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgTierUndelegate{
				Owner:      "invalid",
				PositionId: 1,
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgTierRedelegate_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()
	validValidator := sdk.ValAddress([]byte("test_validator______")).String()

	tests := []struct {
		name        string
		msg         types.MsgTierRedelegate
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgTierRedelegate{
				Owner:        validOwner,
				PositionId:   1,
				DstValidator: validValidator,
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgTierRedelegate{
				Owner:        "invalid",
				PositionId:   1,
				DstValidator: validValidator,
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "invalid destination validator",
			msg: types.MsgTierRedelegate{
				Owner:        validOwner,
				PositionId:   1,
				DstValidator: "invalid",
			},
			wantErr:     true,
			errContains: "invalid destination validator address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgAddToTierPosition_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgAddToTierPosition
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgAddToTierPosition{
				Owner:      validOwner,
				PositionId: 1,
				Amount:     sdkmath.NewInt(1000),
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgAddToTierPosition{
				Owner:      "invalid",
				PositionId: 1,
				Amount:     sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "zero amount",
			msg: types.MsgAddToTierPosition{
				Owner:      validOwner,
				PositionId: 1,
				Amount:     sdkmath.ZeroInt(),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
		{
			name: "negative amount",
			msg: types.MsgAddToTierPosition{
				Owner:      validOwner,
				PositionId: 1,
				Amount:     sdkmath.NewInt(-1),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgTriggerExitFromTier_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgTriggerExitFromTier
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgTriggerExitFromTier{
				Owner:      validOwner,
				PositionId: 1,
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgTriggerExitFromTier{
				Owner:      "invalid",
				PositionId: 1,
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgClearPosition_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgClearPosition
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgClearPosition{
				Owner:      validOwner,
				PositionId: 1,
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgClearPosition{
				Owner:      "invalid",
				PositionId: 1,
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func makePositionIds(n int) []uint64 {
	ids := make([]uint64, n)
	for i := range ids {
		ids[i] = uint64(i + 1)
	}
	return ids
}

func TestMsgClaimTierRewards_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgClaimTierRewards
		wantErr     bool
		errContains string
	}{
		{
			name: "valid single position",
			msg: types.MsgClaimTierRewards{
				Owner:       validOwner,
				PositionIds: []uint64{1},
			},
		},
		{
			name: "valid multiple positions",
			msg: types.MsgClaimTierRewards{
				Owner:       validOwner,
				PositionIds: []uint64{1, 2, 3},
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgClaimTierRewards{
				Owner:       "invalid",
				PositionIds: []uint64{1},
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "empty position_ids",
			msg: types.MsgClaimTierRewards{
				Owner:       validOwner,
				PositionIds: []uint64{},
			},
			wantErr:     true,
			errContains: "must not be empty",
		},
		{
			name: "nil position_ids",
			msg: types.MsgClaimTierRewards{
				Owner: validOwner,
			},
			wantErr:     true,
			errContains: "must not be empty",
		},
		{
			name: "duplicate position_ids",
			msg: types.MsgClaimTierRewards{
				Owner:       validOwner,
				PositionIds: []uint64{1, 2, 1},
			},
			wantErr:     true,
			errContains: "duplicate",
		},
		{
			name: "exactly at max position_ids",
			msg: types.MsgClaimTierRewards{
				Owner:       validOwner,
				PositionIds: makePositionIds(types.MaxClaimPositionIds),
			},
		},
		{
			name: "exceeds max position_ids",
			msg: types.MsgClaimTierRewards{
				Owner:       validOwner,
				PositionIds: makePositionIds(types.MaxClaimPositionIds + 1),
			},
			wantErr:     true,
			errContains: "too many position_ids",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgWithdrawFromTier_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgWithdrawFromTier
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgWithdrawFromTier{
				Owner:      validOwner,
				PositionId: 1,
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgWithdrawFromTier{
				Owner:      "invalid",
				PositionId: 1,
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgExitTierWithDelegation_Validate(t *testing.T) {
	t.Parallel()

	validOwner := sdk.AccAddress([]byte("test_owner__________")).String()

	tests := []struct {
		name        string
		msg         types.MsgExitTierWithDelegation
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			msg: types.MsgExitTierWithDelegation{
				Owner:      validOwner,
				PositionId: 1,
				Amount:     sdkmath.NewInt(1000),
			},
		},
		{
			name: "invalid owner",
			msg: types.MsgExitTierWithDelegation{
				Owner:      "invalid",
				PositionId: 1,
				Amount:     sdkmath.NewInt(1000),
			},
			wantErr:     true,
			errContains: "invalid owner address",
		},
		{
			name: "zero amount",
			msg: types.MsgExitTierWithDelegation{
				Owner:      validOwner,
				PositionId: 1,
				Amount:     sdkmath.ZeroInt(),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
		{
			name: "negative amount",
			msg: types.MsgExitTierWithDelegation{
				Owner:      validOwner,
				PositionId: 1,
				Amount:     sdkmath.NewInt(-1),
			},
			wantErr:     true,
			errContains: "amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
