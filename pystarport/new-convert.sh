#!/bin/bash
OUTPUT=./proto_python

mkdir $OUTPUT

buf generate ../third_party/cosmos-sdk/proto
buf generate buf.build/cosmos/cosmos-proto
buf generate buf.build/cosmos/gogo-proto
buf generate buf.build/tendermint/tendermint
buf generate buf.build/googleapis/googleapis

rm -rf ./tendermint
