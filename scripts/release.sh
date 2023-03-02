#!/bin/bash
set -e

baseurl="."
build_type="tarball"
build_platform="$(nix eval --impure --raw --expr 'builtins.currentSystem')"
ref_name_clean=$(echo "${GITHUB_REF_NAME:=vdevel}" | sed -e 's/[^A-Za-z0-9._-]/_/g')
NETWORK=${NETWORK:-"mainnet"}

build() {
    set -e
    host="$1"
    name="$2"
    if [[ "$NETWORK" == "testnet" ]]; then
        pkg="chain-maind-testnet-${build_type}"
    else
        pkg="chain-maind-${build_type}"
    fi
    if [[ "$host" == "native" ]]; then
        FLAKE="${baseurl}#${pkg}"
    else
        FLAKE="${baseurl}#legacyPackages.${build_platform}.pkgsCross.${host}.chain-main-matrix.${pkg}"
    fi
    echo "building $FLAKE"
    nix build -L "$FLAKE"
    cp result "chain-main_${ref_name_clean:1}_${name}.tar.gz"
}

if [[ "$build_platform" == "x86_64-linux" ]]; then
    hosts="Linux_x86_64,native Linux_arm64,aarch64-multiplatform Windows_x86_64,mingwW64"
elif [[ "$build_platform" == "aarch64-linux" ]]; then
    hosts="Linux_arm64,native Linux_x86_64,gnu64 Windows_x86_64,mingwW64"
elif [[ "$build_platform" == "x86_64-darwin" ]]; then
    hosts="Darwin_x86_64,native Darwin_arm64,aarch64-darwin"
else
    echo "don't support build platform: $build_platform" 
    exit 1
fi

for t in $hosts; do
    IFS=',' read -r name host <<< "${t}"
    build "$host" "$name"
done
