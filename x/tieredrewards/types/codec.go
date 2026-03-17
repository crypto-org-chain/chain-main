package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers concrete types on the LegacyAmino codec.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(Params{}, "chainmain/tieredrewards/Params", nil)
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "chainmain/tieredrewards/MsgUpdateParams")
	// some messages are too long to be registered with amino. so we standardize to remove the module name from the definition
	legacy.RegisterAminoMsg(cdc, &MsgAddTier{}, "chainmain/MsgAddTier")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateTier{}, "chainmain/MsgUpdateTier")
	legacy.RegisterAminoMsg(cdc, &MsgDeleteTier{}, "chainmain/MsgDeleteTier")
	legacy.RegisterAminoMsg(cdc, &MsgLockTier{}, "chainmain/MsgLockTier")
	legacy.RegisterAminoMsg(cdc, &MsgCommitDelegationToTier{}, "chainmain/MsgCommitDelegationToTier")
	legacy.RegisterAminoMsg(cdc, &MsgTierDelegate{}, "chainmain/MsgTierDelegate")
	legacy.RegisterAminoMsg(cdc, &MsgTierUndelegate{}, "chainmain/MsgTierUndelegate")
	legacy.RegisterAminoMsg(cdc, &MsgTierRedelegate{}, "chainmain/MsgTierRedelegate")
	legacy.RegisterAminoMsg(cdc, &MsgAddToTierPosition{}, "chainmain/MsgAddToTierPosition")
	legacy.RegisterAminoMsg(cdc, &MsgTriggerExitFromTier{}, "chainmain/MsgTriggerExitFromTier")
	legacy.RegisterAminoMsg(cdc, &MsgClaimTierRewards{}, "chainmain/MsgClaimTierRewards")
}

// RegisterInterfaces registers the module's interface types.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgAddTier{},
		&MsgUpdateTier{},
		&MsgDeleteTier{},
		&MsgLockTier{},
		&MsgCommitDelegationToTier{},
		&MsgTierDelegate{},
		&MsgTierUndelegate{},
		&MsgTierRedelegate{},
		&MsgAddToTierPosition{},
		&MsgTriggerExitFromTier{},
		&MsgClaimTierRewards{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
