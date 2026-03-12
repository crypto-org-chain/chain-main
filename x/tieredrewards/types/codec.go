package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers concrete types on the LegacyAmino codec.
// Amino names use short prefix "tier/" to stay within the 39-character limit.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(Params{}, "chainmain/tieredrewards/Params", nil)
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "tier/MsgUpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgLockTier{}, "tier/MsgLockTier")
	legacy.RegisterAminoMsg(cdc, &MsgCommitDelegationToTier{}, "tier/MsgCommitDelegation")
	legacy.RegisterAminoMsg(cdc, &MsgAddToTierPosition{}, "tier/MsgAddToPosition")
	legacy.RegisterAminoMsg(cdc, &MsgTierDelegate{}, "tier/MsgTierDelegate")
	legacy.RegisterAminoMsg(cdc, &MsgTierUndelegate{}, "tier/MsgTierUndelegate")
	legacy.RegisterAminoMsg(cdc, &MsgTierRedelegate{}, "tier/MsgTierRedelegate")
	legacy.RegisterAminoMsg(cdc, &MsgTriggerExitFromTier{}, "tier/MsgTriggerExit")
	legacy.RegisterAminoMsg(cdc, &MsgWithdrawFromTier{}, "tier/MsgWithdrawFromTier")
	legacy.RegisterAminoMsg(cdc, &MsgWithdrawTierRewards{}, "tier/MsgWithdrawRewards")
	legacy.RegisterAminoMsg(cdc, &MsgFundTierPool{}, "tier/MsgFundTierPool")
	legacy.RegisterAminoMsg(cdc, &MsgTransferTierPosition{}, "tier/MsgTransferPosition")
}

// RegisterInterfaces registers the module's interface types.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgLockTier{},
		&MsgCommitDelegationToTier{},
		&MsgAddToTierPosition{},
		&MsgTierDelegate{},
		&MsgTierUndelegate{},
		&MsgTierRedelegate{},
		&MsgTriggerExitFromTier{},
		&MsgWithdrawFromTier{},
		&MsgWithdrawTierRewards{},
		&MsgFundTierPool{},
		&MsgTransferTierPosition{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
