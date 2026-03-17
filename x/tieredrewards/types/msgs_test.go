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
			name: "valid without validator",
			msg: types.MsgLockTier{
				Owner:  validOwner,
				Id:     1,
				Amount: sdkmath.NewInt(1000),
			},
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
				TriggerExitImmediately: true,
			},
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
					require.Contains(t, err.Error(), tt.errContains)
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
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
