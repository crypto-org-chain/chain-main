#!/bin/bash
OUTPUT=./proto_python
COSMOS=../../third_party/cosmos-sdk
TENDERMINT=./tendermint
TMP=$(whereis grpc_python_plugin)
PLUGIN="$(cut -d' ' -f2 <<<"$TMP")"
mkdir $OUTPUT
git clone --branch v0.34.10 https://github.com/tendermint/tendermint.git
# cosmos
python -m grpc.tools.protoc --proto_path=$COSMOS/proto --proto_path=$COSMOS/third_party/proto --python_out=$OUTPUT $(find $COSMOS/proto/cosmos -iname "*.proto") --grpc_python_out=$OUTPUT  --plugin=protoc-gen-grpc_python=$PLUGIN
# cosmos third-party
python -m grpc.tools.protoc --proto_path=$COSMOS/proto --proto_path=$COSMOS/third_party/proto --python_out=$OUTPUT $(find $COSMOS/third_party/proto -iname "*.proto") --grpc_python_out=$OUTPUT  --plugin=protoc-gen-grpc_python=$PLUGIN
python -m grpc.tools.protoc --proto_path=$TENDERMINT/proto --proto_path=$TENDERMINT/third_party/proto --python_out=$OUTPUT $(find $TENDERMINT/proto/tendermint -iname "*.proto") --grpc_python_out=$OUTPUT  --plugin=protoc-gen-grpc_python=$PLUGIN
