#!/bin/bash

make build

CHAINID="chainmain-1"
MONIKER="localtestnet"

# localKey address 0x7cb61d4117ae31a12e393a1cfa3bac666481d02e
VAL_KEY="localkey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

# user1 address 0xc6fe5d33615a1c52c08018c47e8bc53646a0e101
USER1_KEY="user1"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

# user2 address 0x963ebdf2e1f8db8707d05fc75bfeffba1b5bac17
USER2_KEY="user2"
USER2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

# remove existing daemon and client
rm -rf ~/.chain-maind*

# Import keys from mnemonics
echo $VAL_MNEMONIC | ./build/chain-maind keys add $VAL_KEY --recover --keyring-backend test
echo $USER1_MNEMONIC | ./build/chain-maind keys add $USER1_KEY --recover --keyring-backend test
echo $USER2_MNEMONIC | ./build/chain-maind keys add $USER2_KEY --recover --keyring-backend test

./build/chain-maind init $MONIKER --chain-id $CHAINID

# Enable IBC
cat $HOME/.chain-maind/config/genesis.json | jq '.app_state.["transfer"].["params"]["receive_enabled"]=true' > $HOME/.chain-maind/config/tmp_genesis.json && mv $HOME/.chain-maind/config/tmp_genesis.json $HOME/.chain-maind/config/genesis.json
cat $HOME/.chain-maind/config/genesis.json | jq '.app_state.["transfer"].["params"]["send_enabled"]=true' > $HOME/.chain-maind/config/tmp_genesis.json && mv $HOME/.chain-maind/config/tmp_genesis.json $HOME/.chain-maind/config/genesis.json


# Change Gov period
cat $HOME/.chain-maind/config/genesis.json | jq '.app_state.["gov"].["deposit_params"]["max_deposit_period"]="600s"' > $HOME/.chain-maind/config/tmp_genesis.json && mv $HOME/.chain-maind/config/tmp_genesis.json $HOME/.chain-maind/config/genesis.json
cat $HOME/.chain-maind/config/genesis.json | jq '.app_state.["gov"].["voting_params"].voting_period="600s"' > $HOME/.chain-maind/config/tmp_genesis.json && mv $HOME/.chain-maind/config/tmp_genesis.json $HOME/.chain-maind/config/genesis.json



# Allocate genesis accounts (cosmos formatted addresses)
./build/chain-maind add-genesis-account "$(./build/chain-maind keys show $VAL_KEY -a --keyring-backend test)" 1000000000000000000000basecro  --keyring-backend test
./build/chain-maind add-genesis-account "$(./build/chain-maind keys show $USER1_KEY -a --keyring-backend test)" 1000000000000000000000basecro --keyring-backend test
./build/chain-maind add-genesis-account "$(./build/chain-maind keys show $USER2_KEY -a --keyring-backend test)" 1000000000000000000000basecro --keyring-backend test

# Sign genesis transaction
./build/chain-maind genesis gentx $VAL_KEY 1000000000000000000basecro --amount=1000000000000000000basecro --chain-id $CHAINID --keyring-backend test

# Collect genesis tx
./build/chain-maind genesis collect-gentxs

# Run this to ensure everything worked and that the genesis file is setup correctly
./build/chain-maind genesis validate

# Start the node (remove the --pruning=nothing flag if historical queries are not needed)
./build/chain-maind start --pruning=nothing --rpc.unsafe --trace --minimum-gas-prices 1000basecro