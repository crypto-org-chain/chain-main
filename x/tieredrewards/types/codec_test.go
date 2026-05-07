package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestRegisterInterfaces_AllMsgTypes verifies that every message type in the
// tieredrewards module is registered in the interface registry. Without
// registration, messages fail to deserialize when wrapped in MsgExec (authz)
// or governance proposals.
func TestRegisterInterfaces_AllMsgTypes(t *testing.T) {
	t.Parallel()

	registry := cdctypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)

	// Every message that implements sdk.Msg must be resolvable from its type URL.
	msgs := []sdk.Msg{
		&types.MsgUpdateParams{},
		&types.MsgAddTier{},
		&types.MsgUpdateTier{},
		&types.MsgDeleteTier{},
		&types.MsgLockTier{},
		&types.MsgCommitDelegationToTier{},
		&types.MsgTierUndelegate{},
		&types.MsgTierRedelegate{},
		&types.MsgAddToTierPosition{},
		&types.MsgTriggerExitFromTier{},
		&types.MsgClearPosition{},
		&types.MsgClaimTierRewards{},
		&types.MsgWithdrawFromTier{},
		&types.MsgExitTierWithDelegation{},
	}

	for _, msg := range msgs {
		any, err := cdctypes.NewAnyWithValue(msg)
		require.NoError(t, err, "failed to pack %T into Any", msg)

		var resolved sdk.Msg
		err = registry.UnpackAny(any, &resolved)
		require.NoError(t, err, "%T should be resolvable from interface registry", msg)
	}
}
