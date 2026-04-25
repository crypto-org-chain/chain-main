#!/bin/bash

# Start the node (remove the --pruning=nothing flag if historical queries are not needed)
./build/chain-maind start --pruning=nothing --rpc.unsafe --trace --minimum-gas-prices 1000basecro
