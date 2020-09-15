package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
}

var (
	amino = codec.NewLegacyAmino()
)

func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()
}
