<br />
<p align="center">
  <img src="assets/logo.svg" alt="Crypto.org Chain" width="400">
</p>
<br />

<p align="center">
  <a href="https://github.com/crypto-org-chain/chain-main/actions/workflows/build.yml"><img label="Build Status" src="https://github.com/crypto-org-chain/chain-main/actions/workflows/build.yml/badge.svg" /></a>
  <a href="https://codecov.io/gh/crypto-org-chain/chain-main"><img label="Code Coverage" src="https://codecov.io/gh/crypto-org-chain/chain-main/branch/master/graph/badge.svg" /></a>
  <a href="https://discord.gg/pahqHz26q4"><img label="Discord" src="https://img.shields.io/discord/783264383978569728.svg?color=7289da&label=Crypto.org%20Chain&logo=discord&style=flat-square" /></a>
</p>

## Table of Contents

- [Table of Contents](#table-of-contents)
- [1. Description](#1-description)
- [2. Contributing](#2-contributing)
- [3. License](#3-license)
- [4. Documentation](#4-documentation)
- [5. Build full node](#5-build-full-node)
    - [1. Nix](#1-nix)
- [6. Start a local Development Network and Node](#6-start-a-local-development-network-and-node)
- [7. Send Your First Transaction](#7-send-your-first-transaction)
- [8. Testing](#8-testing)
- [9. Pystarport Quick Start](#9-pystarport-quick-start)
  - [install latest python (for linux)](#install-latest-python-for-linux)
  - [set path (for linux or for mac)](#set-path-for-linux-or-for-mac)
  - [install pystarport](#install-pystarport)
  - [quick start](#quick-start)
  - [get status](#get-status)
  - [stop all](#stop-all)
- [10. Useful links](#10-useful-links)

<a id="description" />

## 1. Description

**Crypto.org Chain** is a blockchain application built using Cosmos SDK and Tendermint,
intended as a backbone for some of the existing and future Crypto.org ecosystem.

<a id="contributing" />

## 2. Contributing

Please abide by the [Code of Conduct](CODE_OF_CONDUCT.md) in all interactions,
and the [contributing guidelines](CONTRIBUTING.md) when submitting code.

<a id="license" />

## 3. License

[Apache 2.0](./LICENSE)

<a id="documentation" />

## 4. Documentation

Technical documentation can be found in this [Github repository](https://github.com/crypto-org-chain/chain-docs) (you can read it in [this hosted version](https://crypto.org/docs)).

<a id="build" />

## 5. Build full node

```bash
# COSMOS_BUILD_OPTIONS=rocksdb make build
make build
```

By default, the command will build a binary with Mainnet configurations.

To build with different network, provided `NETWORK` variable to `make` command

```bash
NETWORK=testnet make build
```

<a id="nix" />

#### 1. Nix

Nix is a (cross-language) package manager for reproducible builds.
On Linux and macOS, you can [install it as follows](https://nixos.org/download.html) (on Windows 10, you can possibly use the Windows Subsystem for Linux):

```
$ curl -L https://nixos.org/nix/install | sh
```

If you're using a recent Mac with a [T2 chip](https://support.apple.com/en-us/HT208862), you might need to check [nix macos installation](https://nixos.org/manual/nix/stable/#sect-macos-installation) when the command above is not working in your environment.

You can then run:

```
$ make chaindImage
```

Which will build a docker image that contains the full node binary.

Optionally, you can also use a binary cache to speed up the build process:

```
$ nix-env -iA cachix -f https://cachix.org/api/v1/install
$ cachix use crypto-org-chain
```

<a id="start-local-full-node" />

## 6. Start a local Development Network and Node

Please follow this [documentation](https://crypto.org/docs/getting-started/local-devnet.html#devnet-running-latest-development-node) to run a local devnet.

<a id="send-first-transaction" />

## 7. Send Your First Transaction

After setting the local devnet, you may interact with your local blockchain by following this [documentation](https://crypto.org/docs/getting-started/local-devnet.html#interact-with-the-chain).

<a id="testing" />

## 8. Testing

There are different tests that can be executed in the following ways:

- unit tests: `make test`
- simulations: `make test-sim-*` (e.g. `make test-sim-nondeterminism`)
- integrations tests: `make nix-integration-test` (see more details in [their documentation](integration_tests/README.md))

<a id="pystarport" />

## 9. Pystarport Quick Start

you can install pystarport to manage nodes for development.

### install latest python (for linux)

python version should be 3.8 or above.
you can install python like this.

```
git clone git@github.com:python/cpython.git
cd cpython
git checkout tags/v3.9.5
./configure
make
sudo make install
```

### set path (for linux or for mac)
in some cases, if there are multiple python versions, pystarport cannot be found.  
then adjust python path.  
also `$HOME/.local/bin` should be included to the PATH.



```
export PATH=/usr/local/bin:$HOME/.local/bin:$PATH
```

### install pystarport

python3 -m pip install pystarport

### quick start

run two nodes devnet

```
pystarport serve --data=./data --config=./integration_tests/configs/default.yaml
```

### get status

```
pystarport supervisorctl status
```

### stop all

```
pystarport supervisorctl stop all
```

---

<a id="useful-links" />

## 10. Useful links

- [Project Website](http://crypto.org/)
- [Technical Documentation](http://crypto.org/docs)
- Community chatrooms (non-technical): [Discord](https://discord.gg/nsp9JTC) [Telegram](https://t.me/CryptoComOfficial)
- Developer community channel (technical): [![Support Server](https://img.shields.io/discord/783264383978569728.svg?color=7289da&label=Crypto.org%20Chain&logo=discord&style=flat-square)](https://discord.gg/pahqHz26q4)

- [Cosmos SDK documentation](https://docs.cosmos.network)
- [Cosmos Discord](https://discord.gg/W8trcGV)
- [Pystarport](./pystarport/README.md)
