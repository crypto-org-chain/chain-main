// Cronos.com Chain Copyright 2018-present Cronos.com
package types

import (
	newsdkerrors "cosmossdk.io/errors"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// NewGenesisState creates a new genesis state.
func NewGenesisState(collections []Collection) *GenesisState {
	return &GenesisState{
		Collections: collections,
	}
}

// ValidateGenesis performs basic validation of nfts genesis data returning an
// error for any failed validation criteria.
func ValidateGenesis(data GenesisState) error {
	for _, c := range data.Collections {
		// Id is strict but IBC-aware (ibc/{hash} vouchers from x/nft-transfer);
		// Name is free-form. Mirrors the runtime MsgIssueDenom path.
		if err := ValidateDenomIDWithIBC(c.Denom.Id); err != nil {
			return err
		}

		if err := ValidateDenomName(c.Denom.Name); err != nil {
			return err
		}

		for _, nft := range c.NFTs {
			if nft.GetOwner().Empty() {
				return newsdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "missing owner")
			}

			if err := ValidateTokenID(nft.GetID()); err != nil {
				return err
			}

			if err := ValidateTokenURI(nft.GetURI()); err != nil {
				return err
			}
		}
	}
	return nil
}
