// Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
// Modifications Copyright (c) 2021, CRO Protocol Labs ("Crypto.org") (licensed under the Apache License, Version 2.0)
package types_test

import (
	"bytes"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	denomID  = "denom"
	denom    = "denom"
	id       = "id1"
	nftName  = "report"
	address  = CreateTestAddrs(1)[0]
	address2 = CreateTestAddrs(2)[1]
	// nolint: gosec
	tokenURI = "https://google.com/token-1.json"
	// nolint: gosec
	tokenData = "https://google.com/token-1.json"
)

// CreateTestAddrs creates test addresses
func CreateTestAddrs(numAddrs int) []sdk.AccAddress {
	var addresses []sdk.AccAddress // nolint: prealloc
	var buffer bytes.Buffer

	// start at 100 so we can make up to 999 test addresses with valid test addresses
	for i := 100; i < (numAddrs + 100); i++ {
		numString := strconv.Itoa(i)
		buffer.WriteString("A58856F0FD53BF058B4909A21AEC019107BA6") // base address string

		buffer.WriteString(numString)                    // adding on final two digits to make addresses unique
		res, _ := sdk.AccAddressFromHex(buffer.String()) // nolint: errcheck
		bech := res.String()
		addresses = append(addresses, testAddr(buffer.String(), bech))
		buffer.Reset()
	}
	return addresses
}

// for incode address generation
func testAddr(addr string, bech string) sdk.AccAddress {
	res, err := sdk.AccAddressFromHex(addr)
	if err != nil {
		panic(err)
	}
	bechexpected := res.String()
	if bech != bechexpected {
		panic("Bech encoding doesn't match reference")
	}

	bechres, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(bechres, res) {
		panic("Bech decode and hex decode don't match")
	}

	return res
}
