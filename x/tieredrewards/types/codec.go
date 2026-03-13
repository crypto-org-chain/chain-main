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
	legacy.RegisterAminoMsg(cdc, &MsgAddTier{}, "chainmain/tieredrewards/MsgAddTier")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateTier{}, "chainmain/tieredrewards/MsgUpdateTier")
	legacy.RegisterAminoMsg(cdc, &MsgDeleteTier{}, "chainmain/tieredrewards/MsgDeleteTier")
	legacy.RegisterAminoMsg(cdc, &MsgLockTier{}, "chainmain/tieredrewards/MsgLockTier")
	// chainmain/tieredrewards/MsgCommitDelegationToTier is too long to be registered with amino
	legacy.RegisterAminoMsg(cdc, &MsgCommitDelegationToTier{}, "chainmain/MsgCommitDelegationToTier")
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
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
