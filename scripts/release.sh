#!/bin/bash
set -e

baseurl="."
build_type="tarball"
build_platform="$(nix eval --impure --raw --expr 'builtins.currentSystem')"
ref_name_clean=$(echo "${GITHUB_REF_NAME:=vdevel}" | sed -e 's/[^A-Za-z0-9._-]/_/g')
NETWORK=${NETWORK:-"mainnet"}

build() {
    set -e
    network=$1
    host="$2"
    name="$3"
    pkg="chain-maind${network}-${build_type}"
    if [[ "$host" == "native" ]]; then
        if [[ "${build_platform: -6}" == "-linux" ]]; then
            # static link for linux targets
            FLAKE="${baseurl}#legacyPackages.${build_platform}.pkgsStatic.chain-main-matrix.${pkg}"
        else
            FLAKE="${baseurl}#${pkg}"
        fi
    else
        if [[ "$host" == "aarch64-multiplatform" || "$host" == "gnu64" ]]; then
            # static link for linux targets
            FLAKE="${baseurl}#legacyPackages.${build_platform}.pkgsCross.${host}.pkgsStatic.chain-main-matrix.${pkg}"
        else
            FLAKE="${baseurl}#legacyPackages.${build_platform}.pkgsCross.${host}.chain-main-matrix.${pkg}"
        fi
    fi
    echo "building $FLAKE"
    nix build -L "$FLAKE"
    cp result "chain-main_${ref_name_clean:1}${network}_${name}.tar.gz"
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

for network in "" "-testnet"; do
    for t in $hosts; do
        IFS=',' read -r name host <<< "${t}"
        build "$network" "$host" "$name"
    done
done
