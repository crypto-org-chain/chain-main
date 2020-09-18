#!/usr/bin/env sh

##
## Input parameters
##
BINARY=/chain-maind/${BINARY:-chain-maind}
ID=${ID:-0}
LOG=${LOG:-chain-maind.log}

##
## Assert linux binary
##
if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder. Please use the BINARY environment variable if the name of the binary is not 'chain-maind' E.g.: -e BINARY=chain-maind_my_test_version"
	exit 1
fi
BINARY_CHECK="$(file "$BINARY" | grep 'ELF 64-bit LSB executable, x86-64')"
if [ -z "${BINARY_CHECK}" ]; then
	echo "Binary needs to be OS linux, ARCH amd64"
	exit 1
fi

##
## Run binary with all parameters
##
export CHAINMAINDHOME="/chain-maind/node${ID}/.chain-maind"

if [ -d "`dirname ${CHAINMAINDHOME}/${LOG}`" ]; then
  "$BINARY" "$@" --home "$CHAINMAINDHOME"  | tee "${CHAINMAINDHOME}/${LOG}"
else
  "$BINARY" "$@" --home "$CHAINMAINDHOME" 
fi

chmod 777 -R /chain-maind
