#!/usr/bin/env bash

set -e

BASEDIR=$(dirname "$0")

CHAIN_MAIND_BINARY=$(which chain-maind || (echo -e "\033[31mPlease add chain-maind to PATH\033[0m" 1>&2 && exit 1))
CHAIN_MAIND_USER=$USER
CHAIN_MAIND_BINARY_DIR=$(dirname $(which chain-maind))
CHAIN_MAIND_USER_HOME=$(eval echo "~$USER")

sed "s#<CHAIN_MAIND_BINARY>#$CHAIN_MAIND_BINARY#g; s#<CHAIN_MAIND_USER>#$CHAIN_MAIND_USER#g; s#<CHAIN_MAIND_BINARY_DIR>#$CHAIN_MAIND_BINARY_DIR#g; s#<CHAIN_MAIND_USER_HOME>#$CHAIN_MAIND_USER_HOME#g" $BASEDIR/chain-maind.service.template > $BASEDIR/chain-maind.service

echo -e "\033[32mGenerated $BASEDIR/chain-maind.service\033[0m"

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  sudo cp $BASEDIR/chain-maind.service /etc/systemd/system/chain-maind.service
  sudo systemctl daemon-reload
  sudo systemctl enable chain-maind.service
  echo -e "\033[32mCreated /etc/systemd/system/chain-maind.service\033[0m"
else
  echo -e "\033[31mCan only create /etc/systemd/system/chain-maind.service for linux\033[0m" 1>&2
  exit 1
fi