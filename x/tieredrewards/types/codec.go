package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(Params{}, "chainmain/tieredrewards/Params", nil)
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "chainmain/tieredrewards/MsgUpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgAddTier{}, "chainmain/MsgAddTier")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateTier{}, "chainmain/MsgUpdateTier")
	legacy.RegisterAminoMsg(cdc, &MsgDeleteTier{}, "chainmain/MsgDeleteTier")
	legacy.RegisterAminoMsg(cdc, &MsgLockTier{}, "chainmain/MsgLockTier")
	legacy.RegisterAminoMsg(cdc, &MsgCommitDelegationToTier{}, "chainmain/MsgCommitDelegationToTier")
	legacy.RegisterAminoMsg(cdc, &MsgTierUndelegate{}, "chainmain/MsgTierUndelegate")
	legacy.RegisterAminoMsg(cdc, &MsgTierRedelegate{}, "chainmain/MsgTierRedelegate")
	legacy.RegisterAminoMsg(cdc, &MsgAddToTierPosition{}, "chainmain/MsgAddToTierPosition")
	legacy.RegisterAminoMsg(cdc, &MsgTriggerExitFromTier{}, "chainmain/MsgTriggerExitFromTier")
	legacy.RegisterAminoMsg(cdc, &MsgClearPosition{}, "chainmain/MsgClearPosition")
	legacy.RegisterAminoMsg(cdc, &MsgClaimTierRewards{}, "chainmain/MsgClaimTierRewards")
	legacy.RegisterAminoMsg(cdc, &MsgWithdrawFromTier{}, "chainmain/MsgWithdrawFromTier")
	legacy.RegisterAminoMsg(cdc, &MsgExitTierWithDelegation{}, "chainmain/MsgExitTierWithDelegation")
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgAddTier{},
		&MsgUpdateTier{},
		&MsgDeleteTier{},
		&MsgLockTier{},
		&MsgCommitDelegationToTier{},
		&MsgTierUndelegate{},
		&MsgTierRedelegate{},
		&MsgAddToTierPosition{},
		&MsgTriggerExitFromTier{},
		&MsgClearPosition{},
		&MsgClaimTierRewards{},
		&MsgWithdrawFromTier{},
		&MsgExitTierWithDelegation{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
