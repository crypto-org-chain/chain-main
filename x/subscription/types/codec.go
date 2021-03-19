package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers all the necessary types and interfaces for the
// subscription module.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreatePlan{}, "chain-main/MsgCreatePlan", nil)
	cdc.RegisterConcrete(&MsgStopPlan{}, "chain-main/MsgStopPlan", nil)
	cdc.RegisterConcrete(&MsgCreateSubscription{}, "chain-main/MsgCreateSubscription", nil)
	cdc.RegisterConcrete(&MsgStopSubscription{}, "chain-main/MsgStopSubscription", nil)
	cdc.RegisterConcrete(&MsgStopUserSubscription{}, "chain-main/MsgStopUserSubscription", nil)
}

func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreatePlan{},
		&MsgStopPlan{},
		&MsgCreateSubscription{},
		&MsgStopSubscription{},
		&MsgStopUserSubscription{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	amino = codec.NewLegacyAmino()

	// ModuleCdc references the global x/subscription module codec. Note, the codec should
	// ONLY be used in certain instances of tests and for JSON encoding as Amino is
	// still used for that purpose.
	//
	// The actual codec used for serialization should be provided to x/subscription and
	// defined at the application level.
	ModuleCdc = codec.NewAminoCodec(amino)
)

func init() {
	RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
}
