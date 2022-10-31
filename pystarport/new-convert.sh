#!/bin/bash
OUTPUT=./proto_python

mkdir $OUTPUT

git clone --branch v0.34.22 https://github.com/tendermint/tendermint.git

buf generate ../third_party/cosmos-sdk/proto
buf generate ./tendermint/proto
buf generate buf.build/cosmos/cosmos-proto
buf generate buf.build/cosmos/gogo-proto
buf generate buf.build/googleapis/googleapis

rm -rf ./tendermint
