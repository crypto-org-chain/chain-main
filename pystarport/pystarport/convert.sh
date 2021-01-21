#!/bin/bash
OUTPUT=./proto_python
COSMOS=./cosmos-sdk
TENDERMINT=./tendermint
TMP=$(whereis grpc_python_plugin)
PLUGIN="$(cut -d' ' -f2 <<<"$TMP")"
mkdir $OUTPUT
git clone --branch v0.40.1 https://github.com/cosmos/cosmos-sdk.git
git clone --branch v0.34.3 https://github.com/tendermint/tendermint.git
cp -Rf $COSMOS/third_party/proto/* $COSMOS/proto/ 
rm -rf $COSMOS/proto/cosmos/base/tendermint
# cosmos
protoc --proto_path=$COSMOS/proto --proto_path=$COSMOS/third_party/proto --python_out=$OUTPUT $(find $COSMOS/proto/cosmos -iname "*.proto") --grpc_python_out=$OUTPUT  --plugin=protoc-gen-grpc_python=$PLUGIN
# cosmos third-party
protoc --proto_path=$COSMOS/third_party/proto --proto_path=$COSMOS/proto --python_out=$OUTPUT $(find $COSMOS/third_party/proto -iname "*.proto") --grpc_python_out=$OUTPUT  --plugin=protoc-gen-grpc_python=$PLUGIN
# tendermint
protoc --proto_path=$TENDERMINT/proto --proto_path=$TENDERMINT/third_party/proto --python_out=$OUTPUT $(find $TENDERMINT/proto/tendermint -iname "*.proto") --grpc_python_out=$OUTPUT  --plugin=protoc-gen-grpc_python=$PLUGIN
