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
  - [quitck start](#quitck-start)
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

After setting the local devnet, you may interact with the your local blockchain by following this [documentation](https://crypto.org/docs/getting-started/local-devnet.html#interact-with-the-chain).

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

### quitck start

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
0x1A33B3e3a6A90D86C62516197fC6784aa30F1B0D

* 		[0] BYTE
* 		[1] CALLER
* 		[2] 'B3'(Unknown Opcode)
* 		[3] 'e3'(Unknown Opcode)
* 		[4] 'a6'(Unknown Opcode)
* 		[5] 'A9'(Unknown Opcode)
* 		[6] '0D'(Unknown Opcode)
* 		[7] DUP7
* 		[8] 'C6'(Unknown Opcode)
* 		[9] '25'(Unknown Opcode)
* 		[10] AND
* 		[11] NOT

0x680550b70c53ab1/7810944752542dc2812c344

Bytecode:
* 		[9] PUSH9 0x0550b70c53ab1/7810
* 		[10] SWAP5
* 		[11] '47'(Unknown Opcode)
* 		[12] MSTORE
* 		[13] SLOAD
* 		[14] '2d'(Unknown Opcode)
* 		[15] 'c2'(Unknown Opcode)
* 		[16] DUP2
* 		[17] '2c'(Unknown Opcode)
* 		[18] CALLVALUE

0xb179603336b7580f0f845b1366d0c20a00995779

ABI :Contract  0x8c0d8685AfEAf558F271Fd78e75e647f6357869b

[{"inputs":[{"internalType":"string","name":"name_","type":"string"},{"internalType":"string","name":"symbol_","type":"string"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"approved","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"operator","type":"address"},{"indexed":false,"internalType":"bool","name":"approved","type":"bool"}],"name":"ApprovalForAll","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"role","type":"bytes32"},{"indexed":true,"internalType":"bytes32","name":"previousAdminRole","type":"bytes32"},{"indexed":true,"internalType":"bytes32","name":"newAdminRole","type":"bytes32"}],"name":"RoleAdminChanged","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"role","type":"bytes32"},{"indexed":true,"internalType":"address","name":"account","type":"address"},{"indexed":true,"internalType":"address","name":"sender","type":"address"}],"name":"RoleGranted","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"role","type":"bytes32"},{"indexed":true,"internalType":"address","name":"account","type":"address"},{"indexed":true,"internalType":"address","name":"sender","type":"address"}],"name":"RoleRevoked","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Transfer","type":"event"},{"inputs":[],"name":"DEFAULT_ADMIN_ROLE","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"MINTER_ROLE","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"approve","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"getApproved","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"}],"name":"getRoleAdmin","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"uint256","name":"index","type":"uint256"}],"name":"getRoleMember","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"}],"name":"getRoleMemberCount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"grantRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"hasRole","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"operator","type":"address"}],"name":"isApprovedForAll","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address[]","name":"to","type":"address[]"},{"internalType":"uint256[]","name":"tokenId","type":"uint256[]"},{"internalType":"string[]","name":"uri","type":"string[]"}],"name":"mintMultiple","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address[]","name":"to","type":"address[]"},{"internalType":"uint256[]","name":"tokenId","type":"uint256[]"},{"internalType":"string[]","name":"uri","type":"string[]"},{"internalType":"address[][]","name":"recipientAddresses","type":"address[][]"},{"internalType":"uint256[][]","name":"cashbackValues","type":"uint256[][]"}],"name":"mintMultipleCashback","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"string","name":"uri","type":"string"},{"internalType":"address[]","name":"recipientAddresses","type":"address[]"},{"internalType":"uint256[]","name":"cashbackValues","type":"uint256[]"}],"name":"mintWithCashback","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"string","name":"uri","type":"string"}],"name":"mintWithTokenURI","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"ownerOf","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"renounceRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"revokeRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"safeTransfer","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"safeTransferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"bytes","name":"_data","type":"bytes"}],"name":"safeTransferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"operator","type":"address"},{"internalType":"bool","name":"approved","type":"bool"}],"name":"setApprovalForAll","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes4","name":"interfaceId","type":"bytes4"}],"name":"supportsInterface","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"index","type":"uint256"}],"name":"tokenByIndex","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"tokenCashbackRecipients","outputs":[{"internalType":"address[]","name":"","type":"address[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"tokenCashbackValues","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"uint256","name":"index","type":"uint256"}],"name":"tokenOfOwnerByIndex","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"tokenURI","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"tokensOfOwner","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"transferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"uint256","name":"cashbackValue","type":"uint256"}],"name":"updateCashbackForAuthor","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]
