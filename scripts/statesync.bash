#!/bin/bash
# microtick and bitcanna contributed significantly here.
set -uxe

# set environment variables
export GOPATH=~/go
export PATH=$PATH:~/go/bin


# Install chain-maind
go install -tags rocksdb ./...

# MAKE HOME FOLDER AND GET GENESIS
chain-maind init test --home /gaia2/cro 
wget -O /gaia2/cro/config/genesis.json https://github.com/crypto-org-chain/mainnet/raw/main/crypto-org-chain-mainnet-1/genesis.json

INTERVAL=1000

# GET TRUST HASH AND TRUST HEIGHT

LATEST_HEIGHT=$(curl -s https://mainnet.crypto.org/block | jq -r .result.block.header.height);
BLOCK_HEIGHT=$(($LATEST_HEIGHT-$INTERVAL)) 
TRUST_HASH=$(curl -s "https://mainnet.crypto.org/block?height=$BLOCK_HEIGHT" | jq -r .result.block_id.hash)


# TELL USER WHAT WE ARE DOING
echo "TRUST HEIGHT: $BLOCK_HEIGHT"
echo "TRUST HASH: $TRUST_HASH"


# export state sync vars
export CHAIN_MAIND_GRPC_WEB_ADDRESS=127.0.0.1:9876
export CHAIN_MAIND_STATESYNC_ENABLE=true
export CHAIN_MAIND_P2P_MAX_NUM_OUTBOUND_PEERS=200
export CHAIN_MAIND_STATESYNC_RPC_SERVERS="https://mainnet.crypto.org:443,https://mainnet.crypto.org:443"
export CHAIN_MAIND_STATESYNC_TRUST_HEIGHT=$BLOCK_HEIGHT
export CHAIN_MAIND_STATESYNC_TRUST_HASH=$TRUST_HASH
export CHAIN_MAIND_P2P_SEEDS="8dc1863d1d23cf9ad7cbea215c19bcbe8bf39702@p2p.baaa7e56-cc71-4ae4-b4b3-c6a9d4a9596a.cryptodotorg.bison.run:26656,8a7922f3fb3fb4cfe8cb57281b9d159ca7fd29c6@p2p.aef59b2a-d77e-4922-817a-d1eea614aef4.cryptodotorg.bison.run:26656,494d860a2869b90c458b07d4da890539272785c9@p2p.fabc23d9-e0a1-4ced-8cd7-eb3efd6d9ef3.cryptodotorg.bison.run:26656,dc2540dabadb8302da988c95a3c872191061aed2@p2p.7d1b53c0-b86b-44c8-8c02-e3b0e88a4bf7.cryptodotorg.herd.run:26656,"
export CHAIN_MAIND_P2P_PERSISTENT_PEERS="87c3adb7d8f649c51eebe0d3335d8f9e28c362f2@seed-0.crypto.org:26656,e1d7ff02b78044795371beb1cd5fb803f9389256@seed-1.crypto.org:26656"

chain-maind start --db_backend rocksdb --home /gaia2/cro
