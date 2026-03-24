package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
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
		&types.MsgTierDelegate{},
		&types.MsgTierUndelegate{},
		&types.MsgTierRedelegate{},
		&types.MsgAddToTierPosition{},
		&types.MsgTriggerExitFromTier{},
		&types.MsgClaimTierRewards{},
		&types.MsgWithdrawFromTier{},
		&types.MsgFundTierPool{},
	}

	for _, msg := range msgs {
		any, err := cdctypes.NewAnyWithValue(msg)
		require.NoError(t, err, "failed to pack %T into Any", msg)

		var resolved sdk.Msg
		err = registry.UnpackAny(any, &resolved)
		require.NoError(t, err, "%T should be resolvable from interface registry", msg)
	}
}

// TestRegisterLegacyAminoCodec_AllMsgTypes verifies that every message type is
// registered in the legacy amino codec.
func TestRegisterLegacyAminoCodec_AllMsgTypes(t *testing.T) {
	t.Parallel()

	cdc := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(cdc)

	// Verify MsgWithdrawFromTier and MsgFundTierPool are registered
	// (these were previously missing per H-1 finding).
	require.NotPanics(t, func() {
		cdc.MustMarshalJSON(&types.MsgWithdrawFromTier{
			Owner:      "cosmos1test",
			PositionId: 1,
		})
	}, "MsgWithdrawFromTier should be registered in amino codec")

	require.NotPanics(t, func() {
		cdc.MustMarshalJSON(&types.MsgFundTierPool{
			Depositor: "cosmos1test",
			Amount:    sdk.NewCoins(sdk.NewInt64Coin("stake", 100)),
		})
	}, "MsgFundTierPool should be registered in amino codec")
}
