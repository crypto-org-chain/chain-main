#!/bin/bash
OUTPUT=./proto_python

mkdir $OUTPUT

git clone --branch v0.38.x https://github.com/cometbft/cometbft.git

buf generate ../third_party/cosmos-sdk/proto
buf generate ./cometbft/proto
buf generate buf.build/cosmos/cosmos-proto
buf generate buf.build/cosmos/gogo-proto
buf generate buf.build/googleapis/googleapis

rm -rf ./cometbft
